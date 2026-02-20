package resp

import (
	"bufio"
	"errors"
	"io"
	"strconv"
)

var (
	ErrInvalidType      = errors.New("invalid RESP type")
	ErrInvalidFormat    = errors.New("invalid RESP format")
	ErrBulkStringTooBig = errors.New("bulk string exceeds maximum size")
)

const MaxBulkStringSize = 512 * 1024 * 1024

type Reader struct {
	rd *bufio.Reader
}

func NewReader(rd io.Reader) *Reader {
	return &Reader{rd: bufio.NewReader(rd)}
}

func (r *Reader) ReadValue() (*Value, error) {
	b, err := r.rd.ReadByte()
	if err != nil {
		return nil, err
	}

	switch Type(b) {
	case TypeSimpleString:
		return r.readSimpleString()
	case TypeError:
		return r.readError()
	case TypeInteger:
		return r.readInteger()
	case TypeBulkString:
		return r.readBulkString()
	case TypeArray:
		return r.readArray()
	case TypeNull:
		if err := r.readCRLF(); err != nil {
			return nil, err
		}
		return NullValue(), nil
	default:
		return nil, ErrInvalidType
	}
}

func (r *Reader) ReadCommand() (string, [][]byte, error) {
	val, err := r.ReadValue()
	if err != nil {
		return "", nil, err
	}

	if val.Type != TypeArray {
		return "", nil, ErrInvalidFormat
	}

	if val.IsNull || len(val.Array) == 0 {
		return "", nil, ErrInvalidFormat
	}

	cmd := string(val.Array[0].Bulk)
	args := make([][]byte, 0, len(val.Array)-1)
	for i := 1; i < len(val.Array); i++ {
		if val.Array[i].Type == TypeBulkString && !val.Array[i].IsNull {
			args = append(args, val.Array[i].Bulk)
		} else {
			args = append(args, nil)
		}
	}

	return cmd, args, nil
}

func (r *Reader) readSimpleString() (*Value, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	return SimpleString(string(line)), nil
}

func (r *Reader) readError() (*Value, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	return ErrorValue(string(line)), nil
}

func (r *Reader) readInteger() (*Value, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	n, err := strconv.ParseInt(string(line), 10, 64)
	if err != nil {
		return nil, ErrInvalidFormat
	}
	return IntegerValue(n), nil
}

func (r *Reader) readBulkString() (*Value, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}

	size, err := strconv.ParseInt(string(line), 10, 64)
	if err != nil {
		return nil, ErrInvalidFormat
	}

	if size == -1 {
		return NullBulkString(), nil
	}

	if size > MaxBulkStringSize {
		return nil, ErrBulkStringTooBig
	}

	if size == 0 {
		if err := r.readCRLF(); err != nil {
			return nil, err
		}
		return BulkBytes([]byte{}), nil
	}

	buf := make([]byte, size)
	if _, err := io.ReadFull(r.rd, buf); err != nil {
		return nil, err
	}

	if err := r.readCRLF(); err != nil {
		return nil, err
	}

	return BulkBytes(buf), nil
}

func (r *Reader) readArray() (*Value, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}

	count, err := strconv.ParseInt(string(line), 10, 64)
	if err != nil {
		return nil, ErrInvalidFormat
	}

	if count == -1 {
		return NullArray(), nil
	}

	if count == 0 {
		return ArrayValue([]*Value{}), nil
	}

	elements := make([]*Value, 0, count)
	for i := int64(0); i < count; i++ {
		val, err := r.ReadValue()
		if err != nil {
			return nil, err
		}
		elements = append(elements, val)
	}

	return ArrayValue(elements), nil
}

func (r *Reader) readLine() ([]byte, error) {
	var line []byte
	for {
		b, err := r.rd.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == '\r' {
			next, err := r.rd.ReadByte()
			if err != nil {
				return nil, err
			}
			if next == '\n' {
				return line, nil
			}
			line = append(line, b, next)
		} else {
			line = append(line, b)
		}
	}
}

func (r *Reader) readCRLF() error {
	b1, err := r.rd.ReadByte()
	if err != nil {
		return err
	}
	b2, err := r.rd.ReadByte()
	if err != nil {
		return err
	}
	if b1 != '\r' || b2 != '\n' {
		return ErrInvalidFormat
	}
	return nil
}
