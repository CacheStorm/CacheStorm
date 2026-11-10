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
	"unsafe"

	"github.com/cachestorm/cachestorm/internal/store"
)

var ErrDumpChecksum = errors.New("ERR DUMP payload version or checksum are wrong")
var ErrDumpFormat = errors.New("ERR Bad data format")

const maxDumpString = 512 << 20

// The value types Redis has no RDB encoding for that CacheStorm serves are
// dumped under private type bytes, so DUMP/RESTORE and MIGRATE move them
// between CacheStorm nodes whole: a JSON document, a time series, and the
// store's extension values (store.ExtensionBloom and on). Redis refuses
// these payloads. Streams are dumped in Redis' own encoding;
// dumpTypeStream is still read for payloads older CacheStorm versions wrote.
const (
	dumpTypeStream     = 200
	dumpTypeJSON       = 201
	dumpTypeTimeSeries = 202
)

func EncodeDump(value store.Value) ([]byte, error) {
	out, version, err := appendDumpValue(nil, value)
	if err != nil {
		return nil, err
	}
	out = binary.LittleEndian.AppendUint16(out, version)
	out = binary.LittleEndian.AppendUint64(out, rdbChecksum(0, out))
	return out, nil
}

// appendDumpValue appends value's type byte and RDB body to out and returns
// the RDB version the encoding needs. On an error out holds a partial
// encoding the caller drops.
func appendDumpValue(out []byte, value store.Value) ([]byte, uint16, error) {
	version := uint16(11)
	// Redis' own types' strings are saved as rdbSaveRawString saves them;
	// a module value's are plain.
	str := func(s string) { out = appendRDBRawString(out, s) }
	bytesStr := func(b []byte) { out = appendRDBRawString(out, b) }
	plain := func(b []byte) { out = appendDumpLength(out, uint64(len(b))); out = append(out, b...) }
	switch v := value.(type) {
	case *store.StringValue:
		out = append(out, 0)
		bytesStr(v.Data)
	case store.CompactString:
		out = append(out, 0)
		bytesStr(v.Bytes())
	case *store.ListValue:
		v.RLock()
		defer v.RUnlock()
		if v.Len() == 0 {
			return out, 0, ErrDumpFormat
		}
		out = append(out, 1)
		out = appendDumpLength(out, uint64(v.Len()))
		for _, s := range v.Elements() {
			bytesStr(s)
		}
	case *store.SetValue:
		v.RLock()
		defer v.RUnlock()
		if v.Card() == 0 {
			return out, 0, ErrDumpFormat
		}
		out = append(out, 2)
		out = appendDumpLength(out, uint64(v.Card()))
		for _, k := range v.SaveOrder() {
			str(k)
		}
	case *store.HashValue:
		if v.HasFieldTTLs() {
			snap := snapshotHash(v)
			if len(snap.fields) == 0 {
				return out, 0, ErrDumpFormat
			}
			if snap.anyTTL {
				out = appendDumpHashTTL(out, snap)
				version = 12
				break
			}
		}
		v.RLock()
		defer v.RUnlock()
		if v.Len() == 0 {
			return out, 0, ErrDumpFormat
		}
		out = append(out, 4)
		out = appendDumpLength(out, uint64(v.Len()))
		for _, k := range hashFieldOrder(v) {
			str(k)
			bytesStr(v.Field(k))
		}
	case *store.SortedSetValue:
		v.RLock()
		defer v.RUnlock()
		if v.Card() == 0 {
			return out, 0, ErrDumpFormat
		}
		out = append(out, 5)
		out = appendDumpLength(out, uint64(v.Card()))
		keys := make([]string, 0, v.Card())
		for k := range v.All() {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			str(k)
			score, _ := v.GetScore(k)
			if math.IsNaN(score) {
				return out, 0, ErrDumpFormat
			}
			out = binary.LittleEndian.AppendUint64(out, math.Float64bits(score))
		}
	case *store.StreamValue:
		out = appendDumpStream(out, v)
	case *store.JSONValue:
		out = append(out, dumpTypeJSON)
		plain(v.Bytes())
	case *store.TimeSeriesValue:
		var data bytes.Buffer
		v.RLock()
		err := gob.NewEncoder(&data).Encode(v)
		v.RUnlock()
		if err != nil {
			return out, 0, err
		}
		out = append(out, dumpTypeTimeSeries)
		plain(data.Bytes())
	case store.ExtensionValue:
		data, err := v.MarshalBinary()
		if err != nil {
			return out, 0, err
		}
		out = append(out, v.ExtensionType())
		plain(data)
	default:
		raw, ok := value.(store.RedisStringValue)
		if !ok {
			return out, 0, fmt.Errorf("ERR DUMP unsupported value type %T", value)
		}
		data, err := raw.RedisStringBytes()
		if err != nil {
			return out, 0, err
		}
		out = append(out, 0)
		bytesStr(data)
	}
	return out, version, nil
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
	// allExpired is set when a hash failed to decode because every field
	// had expired, which an RDB load skips as Redis does.
	allExpired bool
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
	b, owned := d.raw()
	if owned {
		return unsafeString(b)
	}
	return string(b)
}

// view returns a string's bytes without copying them when they are stored
// plainly: they are then the decoder's input, which the caller copies
// before it keeps them.
func (d *dumpDecoder) view() []byte {
	b, _ := d.raw()
	return b
}

// viewString is view as a string sharing the bytes.
func (d *dumpDecoder) viewString() string { return unsafeString(d.view()) }

// raw decodes a string: its bytes, and whether they were made for the
// call (an integer or a compressed string) rather than being the input.
func (d *dumpDecoder) raw() (_ []byte, owned bool) {
	n, encoded := d.length()
	if d.err != nil {
		return nil, false
	}
	if !encoded {
		if n > maxDumpString {
			d.err = ErrDumpFormat
			return nil, false
		}
		return d.take(int(n)), false
	}
	switch n {
	case 0:
		return strconv.AppendInt(nil, int64(int8(d.byte())), 10), true
	case 1:
		b := d.take(2)
		if len(b) == 2 {
			return strconv.AppendInt(nil, int64(int16(binary.LittleEndian.Uint16(b))), 10), true
		}
	case 2:
		b := d.take(4)
		if len(b) == 4 {
			return strconv.AppendInt(nil, int64(int32(binary.LittleEndian.Uint32(b))), 10), true
		}
	case 3:
		compressed, enc := d.length()
		size, encSize := d.length()
		if enc || encSize || compressed > uint64(len(d.data)-d.pos) || size > uint64(maxDumpString-d.expanded) {
			d.err = ErrDumpFormat
			return nil, false
		}
		input := d.take(int(compressed))
		result, err := decodeDumpLZF(input, int(size))
		if err != nil {
			d.err = err
			return nil, false
		}
		d.expanded += len(result)
		return result, true
	default:
		d.err = ErrDumpFormat
	}
	return nil, false
}

// unsafeString is b as a string sharing its bytes, which must not change.
func unsafeString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
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

func (d *dumpDecoder) value() store.Value { return d.valueOf(d.byte()) }

// valueOf decodes a value of RDB type typ.
func (d *dumpDecoder) valueOf(typ byte) store.Value {
	switch typ {
	case 0:
		// Straight into the form the store keeps a string in: one copy of
		// the bytes instead of a string, a slice and then that form.
		b := d.view()
		if d.err != nil {
			return nil
		}
		return store.NewCompactString(b, store.StringEncAuto)
	case 1:
		n := d.count()
		v := &store.ListValue{}
		for i := 0; i < n && d.err == nil; i++ {
			v.AppendLoaded([]byte(d.str()))
		}
		if n == 0 {
			d.err = ErrDumpFormat
		}
		return v
	case 2:
		n := d.count()
		seen := make(map[string]struct{}, n)
		members := make([]string, 0, n)
		for i := 0; i < n && d.err == nil; i++ {
			s := d.str()
			if _, ok := seen[s]; ok {
				d.err = ErrDumpFormat
			}
			seen[s] = struct{}{}
			members = append(members, s)
		}
		if n == 0 {
			d.err = ErrDumpFormat
		}
		return store.NewLoadedSet(members)
	case 3, 5:
		n := d.count()
		v := store.NewSortedSet(n)
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
			if _, ok := v.GetScore(key); ok || math.IsNaN(score) {
				d.err = ErrDumpFormat
			}
			v.Put(key, score)
		}
		if n == 0 {
			d.err = ErrDumpFormat
		}
		return v
	case 4:
		n := d.count()
		// The fields are read in place; NewLoadedHash copies them.
		items := make([]string, 0, 2*min(n, 1024))
		for i := 0; i < n && d.err == nil; i++ {
			items = append(items, d.viewString(), d.viewString())
		}
		if n == 0 || d.err != nil {
			d.err = ErrDumpFormat
			return nil
		}
		v, ok := store.NewLoadedHash(items)
		if !ok {
			d.err = ErrDumpFormat
			return nil
		}
		return v
	case dumpTypeHashMetadata, dumpTypeHashListpackEx:
		return d.hashWithTTLs(typ)
	case 9, 10, 11, 12, 13, 16, 17, 20:
		// The encoded value is read in place: the decoders copy the
		// elements out, except into hashes, whose elements NewLoadedHash
		// copies itself.
		data := d.view()
		var items []string
		var err error
		switch typ {
		case 11:
			items, err = decodeDumpIntset(data)
		case 9:
			items, err = decodeDumpZipmap(data)
		case 16:
			items, err = decodeDumpListpackIn(data, true)
		case 17, 20:
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
				v.AppendLoaded([]byte(s))
			}
			return v
		case 11, 20:
			seen := make(map[string]struct{}, len(items))
			for _, s := range items {
				if _, ok := seen[s]; ok {
					d.err = ErrDumpFormat
				}
				seen[s] = struct{}{}
			}
			if typ == 20 {
				v := &store.SetValue{}
				v.SetListpackOrder(items)
				return v
			}
			return store.NewLoadedSet(items)
		case 9, 13, 16:
			if len(items)%2 != 0 {
				d.err = ErrDumpFormat
				return nil
			}
			v, ok := store.NewLoadedHash(items)
			if !ok {
				d.err = ErrDumpFormat
				return nil
			}
			return v
		default:
			if len(items)%2 != 0 {
				d.err = ErrDumpFormat
				return nil
			}
			v := store.NewSortedSet(len(items) / 2)
			for i := 0; i < len(items); i += 2 {
				score, err := strconv.ParseFloat(items[i+1], 64)
				if _, ok := v.GetScore(items[i]); ok || err != nil || math.IsNaN(score) {
					d.err = ErrDumpFormat
				}
				v.Put(items[i], score)
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
				v.AppendLoaded([]byte(data))
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
				v.AppendLoaded([]byte(s))
			}
		}
		if v.Len() == 0 {
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
		v, err := store.ParseJSONValue([]byte(data))
		if err != nil {
			d.err = ErrDumpFormat
			return nil
		}
		return v
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
		data := d.str()
		if d.err != nil {
			return nil
		}
		v, ok, err := store.DecodeExtension(typ, []byte(data))
		if !ok || err != nil {
			d.err = ErrDumpFormat
			return nil
		}
		return v
	}
}
