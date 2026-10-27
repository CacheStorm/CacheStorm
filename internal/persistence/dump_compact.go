package persistence

import (
	"encoding/binary"
	"strconv"
)

func dumpSigned(b []byte) int64 {
	var n uint64
	for i, c := range b {
		n |= uint64(c) << uint(8*i)
	}
	if len(b) < 8 && len(b) > 0 && b[len(b)-1]&128 != 0 {
		n |= ^uint64(0) << uint(8*len(b))
	}
	return int64(n)
}

func decodeDumpIntset(data []byte) ([]string, error) {
	if len(data) < 8 {
		return nil, ErrDumpFormat
	}
	width := int(binary.LittleEndian.Uint32(data))
	count := uint64(binary.LittleEndian.Uint32(data[4:]))
	if (width != 2 && width != 4 && width != 8) || count != uint64((len(data)-8)/width) || (len(data)-8)%width != 0 {
		return nil, ErrDumpFormat
	}
	var out []string
	var previous int64
	for pos := 8; pos < len(data); pos += width {
		n := dumpSigned(data[pos : pos+width])
		if len(out) > 0 && n <= previous {
			return nil, ErrDumpFormat
		}
		previous = n
		out = append(out, strconv.FormatInt(n, 10))
	}
	return out, nil
}

func decodeDumpListpack(data []byte) ([]string, error) {
	if len(data) < 7 || uint64(binary.LittleEndian.Uint32(data)) != uint64(len(data)) {
		return nil, ErrDumpFormat
	}
	d := &dumpDecoder{data: data, pos: 6}
	var out []string
	for d.err == nil && d.pos < len(data)-1 {
		start := d.pos
		b := d.byte()
		var s string
		switch {
		case b < 128:
			s = strconv.Itoa(int(b))
		case b < 192:
			s = string(d.take(int(b & 63)))
		case b < 224:
			n := int64(b&31)<<8 | int64(d.byte())
			if n&4096 != 0 {
				n -= 8192
			}
			s = strconv.FormatInt(n, 10)
		case b < 240:
			s = string(d.take(int(b&15)<<8 | int(d.byte())))
		case b == 240:
			n := d.take(4)
			if len(n) == 4 {
				length := uint64(binary.LittleEndian.Uint32(n))
				if length > uint64(len(data)-d.pos) {
					d.err = ErrDumpFormat
				} else {
					s = string(d.take(int(length)))
				}
			}
		case b >= 241 && b <= 244:
			width := []int{2, 3, 4, 8}[b-241]
			s = strconv.FormatInt(dumpSigned(d.take(width)), 10)
		default:
			d.err = ErrDumpFormat
		}
		length := d.pos - start
		n := 5
		switch {
		case length <= 127:
			n = 1
		case length < 16383:
			n = 2
		case length < 2097151:
			n = 3
		case length < 268435455:
			n = 4
		}
		for i := n - 1; i >= 0; i-- {
			want := byte(length >> uint(i*7) & 127)
			if i != n-1 {
				want |= 128
			}
			if d.byte() != want {
				d.err = ErrDumpFormat
			}
		}
		out = append(out, s)
	}
	if d.err != nil || d.byte() != 255 || d.pos != len(data) {
		return nil, ErrDumpFormat
	}
	count := int(binary.LittleEndian.Uint16(data[4:]))
	if count != 65535 && count != len(out) {
		return nil, ErrDumpFormat
	}
	return out, nil
}

func decodeDumpZiplist(data []byte) ([]string, error) {
	if len(data) < 11 || uint64(binary.LittleEndian.Uint32(data)) != uint64(len(data)) {
		return nil, ErrDumpFormat
	}
	d := &dumpDecoder{data: data, pos: 10}
	var out []string
	previous, last := 0, 10
	for d.err == nil && d.pos < len(data)-1 {
		start := d.pos
		prev := int(d.byte())
		if prev == 254 {
			b := d.take(4)
			if len(b) == 4 {
				n := uint64(binary.LittleEndian.Uint32(b))
				if n > uint64(len(data)) {
					d.err = ErrDumpFormat
				} else {
					prev = int(n)
				}
			}
		}
		if prev != previous {
			d.err = ErrDumpFormat
		}
		b := d.byte()
		var s string
		switch {
		case b < 64:
			s = string(d.take(int(b)))
		case b < 128:
			s = string(d.take(int(b&63)<<8 | int(d.byte())))
		case b == 128:
			n := d.take(4)
			if len(n) == 4 {
				length := uint64(binary.BigEndian.Uint32(n))
				if length > uint64(len(data)-d.pos) {
					d.err = ErrDumpFormat
				} else {
					s = string(d.take(int(length)))
				}
			}
		case b == 192:
			s = strconv.FormatInt(dumpSigned(d.take(2)), 10)
		case b == 208:
			s = strconv.FormatInt(dumpSigned(d.take(4)), 10)
		case b == 224:
			s = strconv.FormatInt(dumpSigned(d.take(8)), 10)
		case b == 240:
			s = strconv.FormatInt(dumpSigned(d.take(3)), 10)
		case b == 254:
			s = strconv.FormatInt(dumpSigned(d.take(1)), 10)
		case b >= 241 && b <= 253:
			s = strconv.Itoa(int(b) - 241)
		default:
			d.err = ErrDumpFormat
		}
		previous = d.pos - start
		last = start
		out = append(out, s)
	}
	if d.err != nil || d.byte() != 255 || d.pos != len(data) || uint64(binary.LittleEndian.Uint32(data[4:])) != uint64(last) {
		return nil, ErrDumpFormat
	}
	count := int(binary.LittleEndian.Uint16(data[8:]))
	if count != 65535 && count != len(out) {
		return nil, ErrDumpFormat
	}
	return out, nil
}

func decodeDumpZipmap(data []byte) ([]string, error) {
	if len(data) < 2 {
		return nil, ErrDumpFormat
	}
	d := &dumpDecoder{data: data, pos: 1}
	var out []string
	length := func() int {
		n := int(d.byte())
		if n < 254 {
			return n
		}
		if n == 255 {
			d.err = ErrDumpFormat
			return 0
		}
		b := d.take(4)
		if len(b) != 4 {
			return 0
		}
		v := uint64(binary.LittleEndian.Uint32(b))
		if v > uint64(len(data)-d.pos) {
			d.err = ErrDumpFormat
			return 0
		}
		return int(v)
	}
	for d.err == nil && d.pos < len(data)-1 {
		key := string(d.take(length()))
		n := length()
		free := int(d.byte())
		value := string(d.take(n))
		d.take(free)
		out = append(out, key, value)
	}
	if d.err != nil || d.byte() != 255 || d.pos != len(data) || (data[0] < 254 && int(data[0])*2 != len(out)) {
		return nil, ErrDumpFormat
	}
	return out, nil
}
