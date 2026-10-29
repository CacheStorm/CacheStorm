package persistence

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/store"
)

var ErrDumpChecksum = errors.New("ERR DUMP payload version or checksum are wrong")
var ErrDumpFormat = errors.New("ERR Bad data format")

const maxDumpString = 512 << 20

// The value types Redis has no RDB encoding for that CacheStorm serves are
// dumped under private type bytes, so DUMP/RESTORE and MIGRATE move them
// between CacheStorm nodes whole: a JSON document and a time series. Redis
// refuses these payloads. Streams are dumped in Redis' own encoding;
// dumpTypeStream is still read for payloads older CacheStorm versions wrote.
const (
	dumpTypeStream     = 200
	dumpTypeJSON       = 201
	dumpTypeTimeSeries = 202
)

func EncodeDump(value store.Value) ([]byte, error) {
	var out []byte
	version := uint16(11)
	str := func(s string) { out = appendDumpLength(out, uint64(len(s))); out = append(out, s...) }
	switch v := value.(type) {
	case *store.StringValue:
		out = append(out, 0)
		str(string(v.Data))
	case *store.ListValue:
		v.RLock()
		defer v.RUnlock()
		if len(v.Elements) == 0 {
			return nil, ErrDumpFormat
		}
		out = append(out, 1)
		out = appendDumpLength(out, uint64(len(v.Elements)))
		for _, s := range v.Elements {
			str(string(s))
		}
	case *store.SetValue:
		v.RLock()
		defer v.RUnlock()
		if len(v.Members) == 0 {
			return nil, ErrDumpFormat
		}
		out = append(out, 2)
		out = appendDumpLength(out, uint64(len(v.Members)))
		keys := make([]string, 0, len(v.Members))
		for k := range v.Members {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			str(k)
		}
	case *store.HashValue:
		if v.HasFieldTTLs() {
			snap := snapshotHash(v)
			if len(snap.fields) == 0 {
				return nil, ErrDumpFormat
			}
			if snap.anyTTL {
				out = appendDumpHashTTL(out, snap)
				version = 12
				break
			}
		}
		v.RLock()
		defer v.RUnlock()
		if len(v.Fields) == 0 {
			return nil, ErrDumpFormat
		}
		out = append(out, 4)
		out = appendDumpLength(out, uint64(len(v.Fields)))
		for _, k := range hashFieldOrder(v) {
			str(k)
			str(string(v.Fields[k]))
		}
	case *store.SortedSetValue:
		v.RLock()
		defer v.RUnlock()
		if len(v.Members) == 0 {
			return nil, ErrDumpFormat
		}
		out = append(out, 5)
		out = appendDumpLength(out, uint64(len(v.Members)))
		keys := make([]string, 0, len(v.Members))
		for k := range v.Members {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			str(k)
			score := v.Members[k]
			if math.IsNaN(score) {
				return nil, ErrDumpFormat
			}
			out = binary.LittleEndian.AppendUint64(out, math.Float64bits(score))
		}
	case *store.StreamValue:
		out = appendDumpStream(out, v)
	case *store.JSONValue:
		v.RLock()
		data := string(v.Data)
		v.RUnlock()
		out = append(out, dumpTypeJSON)
		str(data)
	case *store.TimeSeriesValue:
		var data bytes.Buffer
		v.RLock()
		err := gob.NewEncoder(&data).Encode(v)
		v.RUnlock()
		if err != nil {
			return nil, err
		}
		out = append(out, dumpTypeTimeSeries)
		str(data.String())
	default:
		raw, ok := value.(store.RedisStringValue)
		if !ok {
			return nil, fmt.Errorf("ERR DUMP unsupported value type %T", value)
		}
		data, err := raw.RedisStringBytes()
		if err != nil {
			return nil, err
		}
		out = append(out, 0)
		str(string(data))
	}
	out = binary.LittleEndian.AppendUint16(out, version)
	out = binary.LittleEndian.AppendUint64(out, rdbChecksum(0, out))
	return out, nil
}

func appendDumpLength(dst []byte, n uint64) []byte {
	switch {
	case n < 64:
		return append(dst, byte(n))
	case n < 16384:
		return append(dst, byte(n>>8)|0x40, byte(n))
	case n <= math.MaxUint32:
		return binary.BigEndian.AppendUint32(append(dst, 0x80), uint32(n))
	default:
		return binary.BigEndian.AppendUint64(append(dst, 0x81), n)
	}
}

type dumpDecoder struct {
	data     []byte
	pos      int
	err      error
	expanded int
}

func (d *dumpDecoder) take(n int) []byte {
	if d.err != nil {
		return nil
	}
	if n < 0 || n > len(d.data)-d.pos {
		d.err = ErrDumpFormat
		return nil
	}
	b := d.data[d.pos : d.pos+n]
	d.pos += n
	return b
}
func (d *dumpDecoder) byte() byte {
	b := d.take(1)
	if len(b) == 0 {
		return 0
	}
	return b[0]
}
func (d *dumpDecoder) length() (uint64, bool) {
	b := d.byte()
	switch b >> 6 {
	case 0:
		return uint64(b), false
	case 1:
		return uint64(b&63)<<8 | uint64(d.byte()), false
	case 2:
		if b == 0x80 {
			v := d.take(4)
			if len(v) == 4 {
				return uint64(binary.BigEndian.Uint32(v)), false
			}
		}
		if b == 0x81 {
			v := d.take(8)
			if len(v) == 8 {
				return binary.BigEndian.Uint64(v), false
			}
		}
		d.err = ErrDumpFormat
		return 0, false
	default:
		return uint64(b & 63), true
	}
}
func (d *dumpDecoder) count() int {
	n, encoded := d.length()
	if encoded || n > uint64(len(d.data)-d.pos) {
		d.err = ErrDumpFormat
		return 0
	}
	return int(n)
}
func (d *dumpDecoder) str() string {
	n, encoded := d.length()
	if d.err != nil {
		return ""
	}
	if !encoded {
		if n > maxDumpString {
			d.err = ErrDumpFormat
			return ""
		}
		return string(d.take(int(n)))
	}
	switch n {
	case 0:
		return strconv.FormatInt(int64(int8(d.byte())), 10)
	case 1:
		b := d.take(2)
		if len(b) == 2 {
			return strconv.FormatInt(int64(int16(binary.LittleEndian.Uint16(b))), 10)
		}
	case 2:
		b := d.take(4)
		if len(b) == 4 {
			return strconv.FormatInt(int64(int32(binary.LittleEndian.Uint32(b))), 10)
		}
	case 3:
		compressed, enc := d.length()
		size, encSize := d.length()
		if enc || encSize || compressed > uint64(len(d.data)-d.pos) || size > uint64(maxDumpString-d.expanded) {
			d.err = ErrDumpFormat
			return ""
		}
		input := d.take(int(compressed))
		result, err := decodeDumpLZF(input, int(size))
		if err != nil {
			d.err = err
			return ""
		}
		d.expanded += len(result)
		return string(result)
	default:
		d.err = ErrDumpFormat
	}
	return ""
}
func decodeDumpLZF(input []byte, size int) ([]byte, error) {
	out := make([]byte, 0, min(size, len(input)*4+16))
	for pos := 0; pos < len(input); {
		ctrl := int(input[pos])
		pos++
		if ctrl < 32 {
			n := ctrl + 1
			if n > len(input)-pos || n > size-len(out) {
				return nil, ErrDumpFormat
			}
			out = append(out, input[pos:pos+n]...)
			pos += n
			continue
		}
		length := ctrl >> 5
		offset := (ctrl & 31) << 8
		if length == 7 {
			if pos >= len(input) {
				return nil, ErrDumpFormat
			}
			length += int(input[pos])
			pos++
		}
		if pos >= len(input) {
			return nil, ErrDumpFormat
		}
		offset += int(input[pos]) + 1
		pos++
		length += 2
		if offset > len(out) || length > size-len(out) {
			return nil, ErrDumpFormat
		}
		for i := 0; i < length; i++ {
			out = append(out, out[len(out)-offset])
		}
	}
	if len(out) != size {
		return nil, ErrDumpFormat
	}
	return out, nil
}

func DecodeDump(payload []byte) (store.Value, error) {
	if len(payload) < 11 {
		return nil, ErrDumpChecksum
	}
	version := binary.LittleEndian.Uint16(payload[len(payload)-10:])
	if version > 12 || binary.LittleEndian.Uint64(payload[len(payload)-8:]) != rdbChecksum(0, payload[:len(payload)-8]) {
		return nil, ErrDumpChecksum
	}
	d := &dumpDecoder{data: payload[:len(payload)-10]}
	value := d.value()
	if d.err != nil || d.pos != len(d.data) || value == nil {
		return nil, ErrDumpFormat
	}
	return value, nil
}

func (d *dumpDecoder) value() store.Value {
	typ := d.byte()
	switch typ {
	case 0:
		return &store.StringValue{Data: []byte(d.str())}
	case 1:
		n := d.count()
		v := &store.ListValue{}
		for i := 0; i < n && d.err == nil; i++ {
			v.Elements = append(v.Elements, []byte(d.str()))
		}
		if n == 0 {
			d.err = ErrDumpFormat
		}
		return v
	case 2:
		n := d.count()
		v := &store.SetValue{Members: make(map[string]struct{})}
		for i := 0; i < n && d.err == nil; i++ {
			s := d.str()
			if _, ok := v.Members[s]; ok {
				d.err = ErrDumpFormat
			}
			v.Members[s] = struct{}{}
		}
		if n == 0 {
			d.err = ErrDumpFormat
		}
		return v
	case 3, 5:
		n := d.count()
		v := &store.SortedSetValue{Members: make(map[string]float64)}
		for i := 0; i < n && d.err == nil; i++ {
			key := d.str()
			var score float64
			if typ == 5 {
				b := d.take(8)
				if len(b) == 8 {
					score = math.Float64frombits(binary.LittleEndian.Uint64(b))
				}
			} else {
				size := d.byte()
				switch size {
				case 253:
					score = math.NaN()
				case 254:
					score = math.Inf(1)
				case 255:
					score = math.Inf(-1)
				default:
					var err error
					score, err = strconv.ParseFloat(string(d.take(int(size))), 64)
					if err != nil {
						d.err = ErrDumpFormat
					}
				}
			}
			if _, ok := v.Members[key]; ok || math.IsNaN(score) {
				d.err = ErrDumpFormat
			}
			v.Members[key] = score
		}
		if n == 0 {
			d.err = ErrDumpFormat
		}
		return v
	case 4:
		n := d.count()
		v := &store.HashValue{Fields: make(map[string][]byte)}
		order := make([]string, 0, n)
		for i := 0; i < n && d.err == nil; i++ {
			key := d.str()
			s := d.str()
			if _, ok := v.Fields[key]; ok {
				d.err = ErrDumpFormat
			}
			v.Fields[key] = []byte(s)
			order = append(order, key)
		}
		if n == 0 {
			d.err = ErrDumpFormat
		}
		v.SetOrder(order)
		return v
	case dumpTypeHashMetadata, dumpTypeHashListpackEx:
		return d.hashWithTTLs(typ)
	case 9, 10, 11, 12, 13, 16, 17, 20:
		data := []byte(d.str())
		var items []string
		var err error
		switch typ {
		case 11:
			items, err = decodeDumpIntset(data)
		case 9:
			items, err = decodeDumpZipmap(data)
		case 16, 17, 20:
			items, err = decodeDumpListpack(data)
		default:
			items, err = decodeDumpZiplist(data)
		}
		if err != nil || len(items) == 0 {
			d.err = ErrDumpFormat
			return nil
		}
		switch typ {
		case 10:
			v := &store.ListValue{}
			for _, s := range items {
				v.Elements = append(v.Elements, []byte(s))
			}
			return v
		case 11, 20:
			v := &store.SetValue{Members: make(map[string]struct{})}
			for _, s := range items {
				if _, ok := v.Members[s]; ok {
					d.err = ErrDumpFormat
				}
				v.Members[s] = struct{}{}
			}
			return v
		case 9, 13, 16:
			if len(items)%2 != 0 {
				d.err = ErrDumpFormat
				return nil
			}
			v := &store.HashValue{Fields: make(map[string][]byte)}
			order := make([]string, 0, len(items)/2)
			for i := 0; i < len(items); i += 2 {
				if _, ok := v.Fields[items[i]]; ok {
					d.err = ErrDumpFormat
				}
				v.Fields[items[i]] = []byte(items[i+1])
				order = append(order, items[i])
			}
			v.SetOrder(order)
			return v
		default:
			if len(items)%2 != 0 {
				d.err = ErrDumpFormat
				return nil
			}
			v := &store.SortedSetValue{Members: make(map[string]float64)}
			for i := 0; i < len(items); i += 2 {
				score, err := strconv.ParseFloat(items[i+1], 64)
				if _, ok := v.Members[items[i]]; ok || err != nil || math.IsNaN(score) {
					d.err = ErrDumpFormat
				}
				v.Members[items[i]] = score
			}
			return v
		}
	case 14, 18:
		n := d.count()
		v := &store.ListValue{}
		for i := 0; i < n && d.err == nil; i++ {
			container := uint64(2)
			if typ == 18 {
				var enc bool
				container, enc = d.length()
				if enc || container < 1 || container > 2 {
					d.err = ErrDumpFormat
					break
				}
			}
			data := d.str()
			if container == 1 {
				v.Elements = append(v.Elements, []byte(data))
				continue
			}
			var items []string
			var err error
			if typ == 18 {
				items, err = decodeDumpListpack([]byte(data))
			} else {
				items, err = decodeDumpZiplist([]byte(data))
			}
			if err != nil {
				d.err = err
				break
			}
			for _, s := range items {
				v.Elements = append(v.Elements, []byte(s))
			}
		}
		if len(v.Elements) == 0 {
			d.err = ErrDumpFormat
		}
		return v
	case rdbTypeStreamListpacks, rdbTypeStreamListpacks2, rdbTypeStreamListpacks3:
		return d.stream(typ)
	case dumpTypeStream:
		data := d.str()
		if d.err != nil {
			return nil
		}
		v := store.NewStreamValue(0)
		if err := gob.NewDecoder(strings.NewReader(data)).Decode(v); err != nil {
			d.err = ErrDumpFormat
			return nil
		}
		if v.Groups == nil {
			v.Groups = make(map[string]*store.ConsumerGroup)
		}
		return v
	case dumpTypeJSON:
		data := d.str()
		if d.err != nil {
			return nil
		}
		return &store.JSONValue{Data: []byte(data)}
	case dumpTypeTimeSeries:
		data := d.str()
		if d.err != nil {
			return nil
		}
		v := store.NewTimeSeriesValue(0)
		if err := gob.NewDecoder(strings.NewReader(data)).Decode(v); err != nil {
			d.err = ErrDumpFormat
			return nil
		}
		if v.Labels == nil {
			v.Labels = make(map[string]string)
		}
		return v
	default:
		d.err = ErrDumpFormat
		return nil
	}
}
