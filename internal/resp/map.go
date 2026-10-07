package resp

import "strconv"

func (r *Reader) readMap() (*Value, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	count, err := strconv.ParseUint(string(line), 10, 64)
	if err != nil {
		return nil, ErrInvalidFormat
	}
	if count > MaxArrayElements {
		return nil, ErrArrayTooLarge
	}
	values := make(map[string]*Value)
	for i := uint64(0); i < count; i++ {
		key, err := r.ReadValue()
		if err != nil {
			return nil, err
		}
		var name string
		switch key.Type {
		case TypeSimpleString:
			name = key.Str
		case TypeBulkString:
			if key.IsNull {
				return nil, ErrInvalidFormat
			}
			name = string(key.Bulk)
		default:
			return nil, ErrInvalidFormat
		}
		value, err := r.ReadValue()
		if err != nil {
			return nil, err
		}
		values[name] = value
	}
	return MapValue(values), nil
}

func (w *Writer) writeMapNoFlush(values map[string]*Value) error {
	if err := w.wr.WriteByte(byte(TypeMap)); err != nil {
		return err
	}
	if _, err := w.wr.WriteString(strconv.Itoa(len(values))); err != nil {
		return err
	}
	if _, err := w.wr.WriteString("\r\n"); err != nil {
		return err
	}
	for key, value := range values {
		if value == nil {
			return ErrInvalidType
		}
		if err := w.WriteValueNoFlush(BulkString(key)); err != nil {
			return err
		}
		if err := w.WriteValueNoFlush(value); err != nil {
			return err
		}
	}
	return nil
}
