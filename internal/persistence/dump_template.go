package persistence

import (
	"errors"
	"io"
	"strconv"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Redis 8.10 keeps the hashes HIMPORT creates in template encodings: the
// field names live once in a shared template and each hash holds only the
// values. A DUMP payload carries the field names with each hash (types 29
// and 31); an RDB file writes every template once (opcode 242) and the
// hashes name it by id (types 30 and 32). CacheStorm loads them as ordinary
// hashes.
const (
	rdbTypeHashTmplLp       = 29
	rdbTypeHashTmplLpRef    = 30
	rdbTypeHashTmplArray    = 31
	rdbTypeHashTmplArrayRef = 32
)

var errCorruptTemplate = errors.New("corrupt hash template")

// templateFieldsLess orders field names as Redis' sdscmplen: by length,
// then bytes. A template's fields are strictly in that order.
func templateFieldsLess(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

func validTemplateFields(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	for i := 1; i < len(fields); i++ {
		if !templateFieldsLess(fields[i-1], fields[i]) {
			return false
		}
	}
	return true
}

// templateFields reads the field names of a self-contained template hash:
// a format, then a listpack of the names (0) or a count and the names (1).
func (d *dumpDecoder) templateFields() []string {
	format, encoded := d.length()
	if encoded {
		d.err = ErrDumpFormat
	}
	if d.err != nil {
		return nil
	}
	var fields []string
	switch format {
	case 0:
		var err error
		if fields, err = decodeDumpListpack(d.view()); err != nil {
			d.err = ErrDumpFormat
			return nil
		}
	case 1:
		n := d.count()
		for i := 0; i < n && d.err == nil; i++ {
			fields = append(fields, d.str())
		}
	default:
		d.err = ErrDumpFormat
		return nil
	}
	if d.err == nil && !validTemplateFields(fields) {
		d.err = ErrDumpFormat
	}
	return fields
}

// templateValues reads a template listpack: the template id, then one value
// per field.
func (d *dumpDecoder) templateValues() (id uint64, values []string) {
	items, err := decodeDumpListpack(d.view())
	if err != nil || len(items) < 2 {
		d.err = ErrDumpFormat
		return 0, nil
	}
	id, err = strconv.ParseUint(items[0], 10, 64)
	if err != nil {
		d.err = ErrDumpFormat
	}
	return id, items[1:]
}

// templateHash decodes a hash of template type typ.
func (d *dumpDecoder) templateHash(typ byte) store.Value {
	var fields, values []string
	switch typ {
	case rdbTypeHashTmplLp:
		fields = d.templateFields()
		if d.err != nil {
			return nil
		}
		_, values = d.templateValues()
	case rdbTypeHashTmplArray:
		fields = d.templateFields()
		for i := 0; i < len(fields) && d.err == nil; i++ {
			values = append(values, d.str())
		}
	case rdbTypeHashTmplLpRef, rdbTypeHashTmplArrayRef:
		// Only an RDB file holds the templates these name.
		if d.templates == nil {
			d.err = ErrDumpFormat
			return nil
		}
		var id uint64
		if typ == rdbTypeHashTmplLpRef {
			id, values = d.templateValues()
		} else {
			var encoded bool
			if id, encoded = d.length(); encoded {
				d.err = ErrDumpFormat
			}
			if d.err != nil {
				return nil
			}
		}
		var known bool
		if fields, known = d.templates[id]; !known {
			d.err = ErrDumpFormat
			return nil
		}
		if typ == rdbTypeHashTmplArrayRef {
			for i := 0; i < len(fields) && d.err == nil; i++ {
				values = append(values, d.str())
			}
		}
	}
	if d.err != nil || len(values) != len(fields) {
		d.err = ErrDumpFormat
		return nil
	}
	items := make([]string, 0, 2*len(fields))
	for i, f := range fields {
		items = append(items, f, values[i])
	}
	v, ok := store.NewLoadedHash(items)
	if !ok {
		d.err = ErrDumpFormat
		return nil
	}
	return v
}

// readHashTemplate reads one template record of a Redis 8.10 RDB file (the
// opcode is already read): its id, field count and field names.
func (r *RDBReader) readHashTemplate(f io.Reader) error {
	id, err := r.readLength(f)
	if err != nil {
		return err
	}
	n, err := r.readLength(f)
	if err != nil {
		return err
	}
	if n <= 0 {
		return errCorruptTemplate
	}
	fields := make([]string, 0, min(n, 1024))
	for i := 0; i < n; i++ {
		field, err := r.readString(f)
		if err != nil {
			return err
		}
		fields = append(fields, field)
	}
	if !validTemplateFields(fields) {
		return errCorruptTemplate
	}
	if r.hashTemplates == nil {
		r.hashTemplates = make(map[uint64][]string)
	}
	if _, dup := r.hashTemplates[uint64(id)]; dup {
		return errCorruptTemplate
	}
	r.hashTemplates[uint64(id)] = fields
	return nil
}
