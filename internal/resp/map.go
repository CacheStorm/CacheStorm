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
	if r.depth >= MaxNestingDepth {
		return nil, ErrInvalidFormat
	}
	r.depth++
	defer func() { r.depth-- }()

	// The pairs keep the server's order and key types.
	pairs := make([]*Value, 0, 2*count)
	for i := uint64(0); i < 2*count; i++ {
		v, err := r.ReadValue()
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, v)
	}
	return PairsValue(pairs), nil
}

func (w *Writer) writeMapNoFlush(values map[string]*Value) error {
	header, n := TypeMap, len(values)
	if !w.resp3.Load() {
		header, n = TypeArray, 2*len(values)
	}
	if err := w.wr.WriteByte(byte(header)); err != nil {
		return err
	}
	if _, err := w.wr.WriteString(strconv.Itoa(n)); err != nil {
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

// writePairsNoFlush writes an ordered map held as key, value, ... pairs.
func (w *Writer) writePairsNoFlush(pairs []*Value) error {
	if len(pairs)%2 != 0 {
		return ErrInvalidType
	}
	header, n := TypeMap, len(pairs)/2
	if !w.resp3.Load() {
		header, n = TypeArray, len(pairs)
	}
	w.wr.WriteByte(byte(header))
	w.wr.WriteString(strconv.Itoa(n))
	if _, err := w.wr.WriteString("\r\n"); err != nil {
		return err
	}
	for _, v := range pairs {
		if err := w.WriteValueNoFlush(v); err != nil {
			return err
		}
	}
	return nil
}
