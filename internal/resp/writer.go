package resp

import (
	"bufio"
	"io"
	"strconv"
)

type Writer struct {
	wr *bufio.Writer
}

func NewWriter(wr io.Writer) *Writer {
	return &Writer{wr: bufio.NewWriter(wr)}
}

func (w *Writer) Flush() error {
	return w.wr.Flush()
}

func (w *Writer) WriteValue(v *Value) error {
	switch v.Type {
	case TypeSimpleString:
		return w.WriteSimpleString(v.Str)
	case TypeError:
		return w.WriteError(v.Err)
	case TypeInteger:
		return w.WriteInteger(v.Int)
	case TypeBulkString:
		if v.IsNull {
			return w.WriteNullBulkString()
		}
		return w.WriteBulkBytes(v.Bulk)
	case TypeArray:
		if v.IsNull {
			return w.WriteNullArray()
		}
		return w.WriteArray(v.Array)
	case TypeNull:
		return w.WriteNull()
	default:
		return ErrInvalidType
	}
}

func (w *Writer) WriteSimpleString(s string) error {
	if err := w.wr.WriteByte(byte(TypeSimpleString)); err != nil {
		return err
	}
	if _, err := w.wr.WriteString(s); err != nil {
		return err
	}
	if _, err := w.wr.WriteString("\r\n"); err != nil {
		return err
	}
	return w.wr.Flush()
}

func (w *Writer) WriteError(s string) error {
	if err := w.wr.WriteByte(byte(TypeError)); err != nil {
		return err
	}
	if _, err := w.wr.WriteString(s); err != nil {
		return err
	}
	if _, err := w.wr.WriteString("\r\n"); err != nil {
		return err
	}
	return w.wr.Flush()
}

func (w *Writer) WriteInteger(n int64) error {
	if err := w.wr.WriteByte(byte(TypeInteger)); err != nil {
		return err
	}
	if _, err := w.wr.WriteString(strconv.FormatInt(n, 10)); err != nil {
		return err
	}
	if _, err := w.wr.WriteString("\r\n"); err != nil {
		return err
	}
	return w.wr.Flush()
}

func (w *Writer) WriteBulkString(s string) error {
	return w.WriteBulkBytes([]byte(s))
}

func (w *Writer) WriteBulkBytes(b []byte) error {
	if err := w.wr.WriteByte(byte(TypeBulkString)); err != nil {
		return err
	}
	if _, err := w.wr.WriteString(strconv.Itoa(len(b))); err != nil {
		return err
	}
	if _, err := w.wr.WriteString("\r\n"); err != nil {
		return err
	}
	if _, err := w.wr.Write(b); err != nil {
		return err
	}
	if _, err := w.wr.WriteString("\r\n"); err != nil {
		return err
	}
	return w.wr.Flush()
}

func (w *Writer) WriteNullBulkString() error {
	w.wr.WriteByte(byte(TypeBulkString))
	w.wr.WriteString("-1\r\n")
	return w.wr.Flush()
}

func (w *Writer) WriteNull() error {
	w.wr.WriteByte(byte(TypeNull))
	w.wr.WriteString("\r\n")
	return w.wr.Flush()
}

func (w *Writer) WriteNullArray() error {
	w.wr.WriteByte(byte(TypeArray))
	w.wr.WriteString("-1\r\n")
	return w.wr.Flush()
}

func (w *Writer) WriteArray(items []*Value) error {
	w.wr.WriteByte(byte(TypeArray))
	w.wr.WriteString(strconv.Itoa(len(items)))
	w.wr.WriteString("\r\n")
	for _, item := range items {
		if err := w.WriteValueNoFlush(item); err != nil {
			return err
		}
	}
	return w.wr.Flush()
}

func (w *Writer) WriteValueNoFlush(v *Value) error {
	switch v.Type {
	case TypeSimpleString:
		w.wr.WriteByte(byte(TypeSimpleString))
		w.wr.WriteString(v.Str)
		w.wr.WriteString("\r\n")
		return nil
	case TypeError:
		w.wr.WriteByte(byte(TypeError))
		w.wr.WriteString(v.Err)
		w.wr.WriteString("\r\n")
		return nil
	case TypeInteger:
		w.wr.WriteByte(byte(TypeInteger))
		w.wr.WriteString(strconv.FormatInt(v.Int, 10))
		w.wr.WriteString("\r\n")
		return nil
	case TypeBulkString:
		if v.IsNull {
			w.wr.WriteByte(byte(TypeBulkString))
			w.wr.WriteString("-1\r\n")
			return nil
		}
		w.wr.WriteByte(byte(TypeBulkString))
		w.wr.WriteString(strconv.Itoa(len(v.Bulk)))
		w.wr.WriteString("\r\n")
		w.wr.Write(v.Bulk)
		w.wr.WriteString("\r\n")
		return nil
	case TypeArray:
		if v.IsNull {
			w.wr.WriteByte(byte(TypeArray))
			w.wr.WriteString("-1\r\n")
			return nil
		}
		w.wr.WriteByte(byte(TypeArray))
		w.wr.WriteString(strconv.Itoa(len(v.Array)))
		w.wr.WriteString("\r\n")
		for _, item := range v.Array {
			if err := w.WriteValueNoFlush(item); err != nil {
				return err
			}
		}
		return nil
	case TypeNull:
		w.wr.WriteByte(byte(TypeNull))
		w.wr.WriteString("\r\n")
		return nil
	default:
		return ErrInvalidType
	}
}

func (w *Writer) WriteOK() error {
	return w.WriteSimpleString("OK")
}

func (w *Writer) WritePONG() error {
	return w.WriteSimpleString("PONG")
}

func (w *Writer) WriteQueued() error {
	return w.WriteSimpleString("QUEUED")
}

func (w *Writer) WriteZero() error {
	return w.WriteInteger(0)
}

func (w *Writer) WriteOne() error {
	return w.WriteInteger(1)
}
