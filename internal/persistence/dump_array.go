package persistence

import (
	"encoding/binary"
	"math"
	"strconv"

	"github.com/cachestorm/cachestorm/internal/sparsearray"
	"github.com/cachestorm/cachestorm/internal/store"
)

// rdbTypeArray is Redis 8.8's array type.
const rdbTypeArray = 28

// The tags of an array element in an RDB file (AR_RDB_TAG_*).
const (
	arrayTagString      = 0
	arrayTagInt         = 1
	arrayTagFloat       = 2
	arrayTagSmallString = 3
)

// appendDumpArray appends an array as Redis 8.8's rdbSaveObject saves it:
// the element count, the insert cursor (0, or 1 and the cursor), then each
// element in index order with its index, tag and value.
func appendDumpArray(out []byte, v *store.ArrayValue) ([]byte, error) {
	v.RLock()
	defer v.RUnlock()
	ar := v.Array()
	if ar.Count() == 0 {
		return out, ErrDumpFormat
	}
	out = append(out, rdbTypeArray)
	out = appendDumpLength(out, ar.Count())
	if ar.InsertIdx == sparsearray.InsertIdxNone {
		out = appendDumpLength(out, 0)
	} else {
		out = appendDumpLength(appendDumpLength(out, 1), ar.InsertIdx)
	}
	ar.Scan(0, math.MaxUint64-1, func(idx uint64, s string) bool {
		out = appendDumpLength(out, idx)
		switch kind, i, f := store.ClassifyArrayValue(s); kind {
		case store.ArrayInt:
			out = appendDumpLength(out, arrayTagInt)
			out = binary.LittleEndian.AppendUint64(out, uint64(i))
		case store.ArrayFloat:
			out = appendDumpLength(out, arrayTagFloat)
			out = binary.LittleEndian.AppendUint64(out, math.Float64bits(f))
		case store.ArraySmallString:
			out = appendDumpLength(out, arrayTagSmallString)
			out = appendRDBRawString(out, s)
		default:
			out = appendDumpLength(out, arrayTagString)
			out = appendRDBRawString(out, s)
		}
		return true
	})
	return out, nil
}

// array reads an array as Redis 8.8's rdbLoadObject does, refusing what
// it reports as corrupt.
func (d *dumpDecoder) array() store.Value {
	count, enc := d.length()
	if d.err != nil || enc || count == 0 {
		d.err = ErrDumpFormat
		return nil
	}
	flag, enc := d.length()
	if enc || flag > 1 {
		d.err = ErrDumpFormat
		return nil
	}
	v := store.NewArrayValue()
	ar := v.Array()
	if flag == 1 {
		ar.InsertIdx, _ = d.length()
	}
	for i := uint64(0); i < count && d.err == nil; i++ {
		idx, enc := d.length()
		if d.err != nil || enc || idx == math.MaxUint64 {
			d.err = ErrDumpFormat
			return nil
		}
		tag, _ := d.length()
		var s string
		switch tag {
		case arrayTagInt:
			b := d.take(8)
			if len(b) != 8 {
				d.err = ErrDumpFormat
				return nil
			}
			s = strconv.FormatInt(int64(binary.LittleEndian.Uint64(b)), 10)
		case arrayTagFloat:
			b := d.take(8)
			if len(b) != 8 {
				d.err = ErrDumpFormat
				return nil
			}
			s = store.FormatArrayFloat(math.Float64frombits(binary.LittleEndian.Uint64(b)))
		case arrayTagSmallString, arrayTagString:
			s = d.str()
			if tag == arrayTagSmallString && len(s) > store.ArraySmallStringMax {
				d.err = ErrDumpFormat
				return nil
			}
		default:
			d.err = ErrDumpFormat
			return nil
		}
		if d.err != nil {
			return nil
		}
		ar.Set(idx, s)
	}
	if d.err != nil {
		return nil
	}
	return v
}
