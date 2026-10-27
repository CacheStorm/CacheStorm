package resp

import (
	"bufio"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
)

var errorLineReplacer = strings.NewReplacer("\r", " ", "\n", " ")

type Writer struct {
	wr *bufio.Writer
	// resp3 selects the RESP3 encodings for null and map replies. A RESP2
	// client cannot parse them, so a writer encodes null as a null bulk
	// string and a map as a flat key/value array until a client opts in.
	resp3 atomic.Bool
	// errors counts the error replies written, so a caller can tell whether
	// a command answered with an error.
	errors int64
}

// Errors returns how many error replies the writer has written.
func (w *Writer) Errors() int64 {
	return w.errors
}

// SetRESP3 switches the writer between RESP2 (the default) and RESP3
// encodings for null and map replies.
func (w *Writer) SetRESP3(on bool) {
	w.resp3.Store(on)
}

// IsRESP3 reports whether the writer uses the RESP3 encodings.
func (w *Writer) IsRESP3() bool {
	return w.resp3.Load()
}

func NewWriter(wr io.Writer) *Writer {
	return &Writer{wr: bufio.NewWriter(wr)}
}

func (w *Writer) Flush() error {
	return w.wr.Flush()
}

func (w *Writer) WriteValue(v *Value) error {
	if v == nil {
		return ErrInvalidType
	}
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
	case TypeMap, TypePush, TypeDouble, TypeVerbatim:
		if err := w.WriteValueNoFlush(v); err != nil {
			return err
		}
		return w.wr.Flush()
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
	if _, err := w.wr.WriteString(errorLineReplacer.Replace(s)); err != nil {
		return err
	}
	if _, err := w.wr.WriteString("\r\n"); err != nil {
		return err
	}
	return w.wr.Flush()
}

func (w *Writer) WriteError(s string) error {
	w.errors++
	if err := w.wr.WriteByte(byte(TypeError)); err != nil {
		return err
	}
	if _, err := w.wr.WriteString(errorLineReplacer.Replace(s)); err != nil {
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
	if w.resp3.Load() {
		return w.WriteNull()
	}
	if err := w.wr.WriteByte(byte(TypeBulkString)); err != nil {
		return err
	}
	if _, err := w.wr.WriteString("-1\r\n"); err != nil {
		return err
	}
	return w.wr.Flush()
}

func (w *Writer) WriteNull() error {
	if !w.resp3.Load() {
		return w.WriteNullBulkString()
	}
	if err := w.wr.WriteByte(byte(TypeNull)); err != nil {
		return err
	}
	if _, err := w.wr.WriteString("\r\n"); err != nil {
		return err
	}
	return w.wr.Flush()
}

func (w *Writer) WriteNullArray() error {
	if w.resp3.Load() {
		return w.WriteNull()
	}
	if err := w.wr.WriteByte(byte(TypeArray)); err != nil {
		return err
	}
	if _, err := w.wr.WriteString("-1\r\n"); err != nil {
		return err
	}
	return w.wr.Flush()
}

func (w *Writer) WriteArray(items []*Value) error {
	if err := w.wr.WriteByte(byte(TypeArray)); err != nil {
		return err
	}
	if _, err := w.wr.WriteString(strconv.Itoa(len(items))); err != nil {
		return err
	}
	if _, err := w.wr.WriteString("\r\n"); err != nil {
		return err
	}
	for _, item := range items {
		if err := w.WriteValueNoFlush(item); err != nil {
			return err
		}
	}
	return w.wr.Flush()
}

func (w *Writer) WriteValueNoFlush(v *Value) error {
	if v == nil {
		return ErrInvalidType
	}
	switch v.Type {
	case TypeSimpleString:
		w.wr.WriteByte(byte(TypeSimpleString))
		w.wr.WriteString(errorLineReplacer.Replace(v.Str))
		_, err := w.wr.WriteString("\r\n")
		return err
	case TypeError:
		w.wr.WriteByte(byte(TypeError))
		w.wr.WriteString(errorLineReplacer.Replace(v.Err))
		_, err := w.wr.WriteString("\r\n")
		return err
	case TypeInteger:
		w.wr.WriteByte(byte(TypeInteger))
		w.wr.WriteString(strconv.FormatInt(v.Int, 10))
		_, err := w.wr.WriteString("\r\n")
		return err
	case TypeBulkString:
		if v.IsNull {
			if w.resp3.Load() {
				_, err := w.wr.WriteString("_\r\n")
				return err
			}
			w.wr.WriteByte(byte(TypeBulkString))
			_, err := w.wr.WriteString("-1\r\n")
			return err
		}
		w.wr.WriteByte(byte(TypeBulkString))
		w.wr.WriteString(strconv.Itoa(len(v.Bulk)))
		w.wr.WriteString("\r\n")
		w.wr.Write(v.Bulk)
		_, err := w.wr.WriteString("\r\n")
		return err
	case TypeArray, TypePush:
		if v.IsNull {
			if w.resp3.Load() {
				_, err := w.wr.WriteString("_\r\n")
				return err
			}
			w.wr.WriteByte(byte(TypeArray))
			_, err := w.wr.WriteString("-1\r\n")
			return err
		}
		header := TypeArray
		if v.Type == TypePush && w.resp3.Load() {
			header = TypePush
		}
		w.wr.WriteByte(byte(header))
		w.wr.WriteString(strconv.Itoa(len(v.Array)))
		if _, err := w.wr.WriteString("\r\n"); err != nil {
			return err
		}
		for _, item := range v.Array {
			if err := w.WriteValueNoFlush(item); err != nil {
				return err
			}
		}
		return nil
	case TypeDouble:
		if !w.resp3.Load() {
			w.wr.WriteByte(byte(TypeBulkString))
			w.wr.WriteString(strconv.Itoa(len(v.Str)))
			w.wr.WriteString("\r\n")
			w.wr.WriteString(v.Str)
			_, err := w.wr.WriteString("\r\n")
			return err
		}
		w.wr.WriteByte(byte(TypeDouble))
		w.wr.WriteString(v.Str)
		_, err := w.wr.WriteString("\r\n")
		return err
	case TypeVerbatim:
		if !w.resp3.Load() {
			w.wr.WriteByte(byte(TypeBulkString))
			w.wr.WriteString(strconv.Itoa(len(v.Bulk)))
		} else {
			w.wr.WriteByte(byte(TypeVerbatim))
			w.wr.WriteString(strconv.Itoa(len(v.Bulk) + 4))
			w.wr.WriteString("\r\n")
			w.wr.WriteString(v.Str)
			w.wr.WriteByte(':')
			w.wr.Write(v.Bulk)
			_, err := w.wr.WriteString("\r\n")
			return err
		}
		w.wr.WriteString("\r\n")
		w.wr.Write(v.Bulk)
		_, err := w.wr.WriteString("\r\n")
		return err
	case TypeMap:
		if v.Map == nil && v.Array != nil {
			return w.writePairsNoFlush(v.Array)
		}
		return w.writeMapNoFlush(v.Map)
	case TypeNull:
		if !w.resp3.Load() {
			w.wr.WriteByte(byte(TypeBulkString))
			_, err := w.wr.WriteString("-1\r\n")
			return err
		}
		w.wr.WriteByte(byte(TypeNull))
		_, err := w.wr.WriteString("\r\n")
		return err
	default:
		return ErrInvalidType
	}
}

// WriteVerbatim writes text as a verbatim string of format, a bulk string
// for a RESP2 client.
func (w *Writer) WriteVerbatim(format, text string) error {
	return w.WriteValue(VerbatimValue(format, text))
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

func (w *Writer) WriteCaptured(data []byte, errors int64) error {
	w.errors += errors
	if _, err := w.wr.Write(data); err != nil {
		return err
	}
	return w.wr.Flush()
}
