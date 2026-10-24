package resp

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
)

var (
	ErrInvalidType      = errors.New("invalid RESP type")
	ErrInvalidFormat    = errors.New("invalid RESP format")
	ErrBulkStringTooBig = errors.New("bulk string exceeds maximum size")
	ErrArrayTooLarge    = errors.New("array exceeds maximum element count")
)

const (
	MaxBulkStringSize = 512 * 1024 * 1024
	MaxArrayElements  = 1048576 // 1M elements max per array
)

type Reader struct {
	rd *bufio.Reader
	// depth is the nesting level of the aggregate being read; ReadValue
	// refuses to go deeper than MaxNestingDepth so a peer cannot drive the
	// recursion into a stack overflow.
	depth int
}

// MaxNestingDepth bounds how deeply arrays and maps may nest in a reply.
const MaxNestingDepth = 128

func NewReader(rd io.Reader) *Reader {
	if br, ok := rd.(*bufio.Reader); ok {
		return &Reader{rd: br}
	}
	return &Reader{rd: bufio.NewReader(rd)}
}

func (r *Reader) Reset(rd io.Reader) {
	if br, ok := rd.(*bufio.Reader); ok {
		r.rd = br
	} else {
		r.rd.Reset(rd)
	}
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
	case TypeMap:
		return r.readMap()
	case TypeNull:
		if err := r.readCRLF(); err != nil {
			return nil, err
		}
		return NullValue(), nil
	default:
		return nil, ErrInvalidType
	}
}

// ReadCommand reads one multibulk command: an array of bulk strings. It is
// the format the AOF holds. The command is parsed flat, without recursing
// into ReadValue, so a nested array is an error instead of a recursion whose
// depth the peer controls, and the header lines are length-bounded.
func (r *Reader) ReadCommand() (string, [][]byte, error) {
	for {
		b, err := r.rd.ReadByte()
		if err != nil {
			return "", nil, err
		}
		if b != byte(TypeArray) {
			return "", nil, ErrInvalidFormat
		}
		cmd, args, err := r.readMultiBulk()
		if err == errEmptyCommand {
			continue
		}
		return cmd, args, err
	}
}

// ReadClientCommand reads one command from a client connection. Besides the
// multibulk form it accepts the inline form Redis supports for telnet-style
// use ("PING\r\n", "SET k v\n"), with Redis' quoting rules. Empty lines are
// skipped, as Redis does.
func (r *Reader) ReadClientCommand() (string, [][]byte, error) {
	for {
		b, err := r.rd.ReadByte()
		if err != nil {
			return "", nil, err
		}
		var cmd string
		var args [][]byte
		if b == byte(TypeArray) {
			cmd, args, err = r.readMultiBulk()
		} else {
			if err := r.rd.UnreadByte(); err != nil {
				return "", nil, err
			}
			cmd, args, err = r.readInline()
		}
		if err == errEmptyCommand {
			continue
		}
		return cmd, args, err
	}
}

// MaxInlineSize bounds an inline command and a multibulk header line, the
// same 64 KiB limit Redis applies.
const MaxInlineSize = 64 * 1024

var (
	errEmptyCommand    = errors.New("empty command")
	ErrLineTooLong     = errors.New("protocol line exceeds maximum length")
	ErrUnbalancedQuote = errors.New("unbalanced quotes in request")
)

func (r *Reader) readMultiBulk() (string, [][]byte, error) {
	line, err := r.readBoundedLine(true)
	if err != nil {
		return "", nil, err
	}
	if len(line) > 0 && line[0] == '-' {
		// A zero or negative count is skipped, as Redis does.
		if _, ok := parseDecimal(line[1:]); !ok {
			return "", nil, ErrInvalidFormat
		}
		return "", nil, errEmptyCommand
	}
	count, ok := parseDecimal(line)
	if !ok {
		return "", nil, ErrInvalidFormat
	}
	if count == 0 {
		return "", nil, errEmptyCommand
	}
	if count > MaxArrayElements {
		return "", nil, ErrArrayTooLarge
	}

	// Grow the argument slice as arguments arrive instead of sizing it from
	// a header the peer chose.
	args := make([][]byte, 0, min(count, 16))
	for i := int64(0); i < count; i++ {
		b, err := r.rd.ReadByte()
		if err != nil {
			return "", nil, unexpectedEOF(err)
		}
		if b != byte(TypeBulkString) {
			return "", nil, ErrInvalidFormat
		}
		arg, err := r.readCommandBulk()
		if err != nil {
			return "", nil, unexpectedEOF(err)
		}
		args = append(args, arg)
	}
	return string(args[0]), args[1:], nil
}

// readCommandBulk reads one bulk-string argument. A large argument is read
// in chunks, so the memory held grows with the bytes actually received and a
// header announcing 512 MB costs nothing until the data arrives.
func (r *Reader) readCommandBulk() ([]byte, error) {
	line, err := r.readBoundedLine(true)
	if err != nil {
		return nil, err
	}
	size, ok := parseDecimal(line)
	if !ok {
		return nil, ErrInvalidFormat
	}
	if size > MaxBulkStringSize {
		return nil, ErrBulkStringTooBig
	}

	const chunk = 1 << 20
	var buf []byte
	if size <= chunk {
		buf = make([]byte, size)
		if _, err := io.ReadFull(r.rd, buf); err != nil {
			return nil, err
		}
	} else {
		var b bytes.Buffer
		b.Grow(chunk)
		if _, err := io.CopyN(&b, r.rd, size); err != nil {
			return nil, err
		}
		buf = b.Bytes()
	}
	if err := r.readCRLF(); err != nil {
		return nil, err
	}
	return buf, nil
}

func (r *Reader) readInline() (string, [][]byte, error) {
	line, err := r.readBoundedLine(false)
	if err != nil {
		return "", nil, err
	}
	args, err := splitInlineArgs(line)
	if err != nil {
		return "", nil, err
	}
	if len(args) == 0 {
		return "", nil, errEmptyCommand
	}
	return string(args[0]), args[1:], nil
}

// readBoundedLine reads one line of at most MaxInlineSize bytes. A multibulk
// header must end in CRLF; an inline command may end in a bare LF, as Redis
// accepts.
func (r *Reader) readBoundedLine(strictCRLF bool) ([]byte, error) {
	var line []byte
	for {
		frag, err := r.rd.ReadSlice('\n')
		if len(line)+len(frag) > MaxInlineSize {
			return nil, ErrLineTooLong
		}
		if err == bufio.ErrBufferFull {
			line = append(line, frag...)
			continue
		}
		if err != nil {
			return nil, unexpectedEOF(err)
		}
		if line == nil {
			line = frag
		} else {
			line = append(line, frag...)
		}
		break
	}
	line = line[:len(line)-1]
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	} else if strictCRLF {
		return nil, ErrInvalidFormat
	}
	if strictCRLF && bytes.IndexByte(line, '\r') >= 0 {
		return nil, ErrInvalidFormat
	}
	return line, nil
}

// parseDecimal parses a non-empty run of ASCII digits. Unlike strconv it
// refuses a sign, so "+3" and "-0" are not lengths.
func parseDecimal(b []byte) (int64, bool) {
	if len(b) == 0 || len(b) > 18 {
		return 0, false
	}
	var n int64
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int64(c-'0')
	}
	return n, true
}

// unexpectedEOF reports a stream that ends inside a command as truncated
// rather than as a clean end of input.
func unexpectedEOF(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// splitInlineArgs splits an inline command the way Redis' sdssplitargs does:
// arguments are separated by spaces, "double quotes" support \n \r \t \b \a
// \\ \" and \xHH escapes, 'single quotes' support \', and a closing quote
// must be followed by a space or the end of the line.
func splitInlineArgs(line []byte) ([][]byte, error) {
	var args [][]byte
	i := 0
	isSpace := func(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f' }
	for {
		for i < len(line) && isSpace(line[i]) {
			i++
		}
		if i >= len(line) {
			return args, nil
		}
		var cur []byte
		inDQ, inSQ := false, false
		for {
			if inDQ {
				if i >= len(line) {
					return nil, ErrUnbalancedQuote
				}
				c := line[i]
				switch {
				case c == '\\' && i+3 < len(line) && line[i+1] == 'x' && isHex(line[i+2]) && isHex(line[i+3]):
					cur = append(cur, hexVal(line[i+2])<<4|hexVal(line[i+3]))
					i += 3
				case c == '\\' && i+1 < len(line):
					i++
					switch line[i] {
					case 'n':
						cur = append(cur, '\n')
					case 'r':
						cur = append(cur, '\r')
					case 't':
						cur = append(cur, '\t')
					case 'b':
						cur = append(cur, '\b')
					case 'a':
						cur = append(cur, '\a')
					default:
						cur = append(cur, line[i])
					}
				case c == '"':
					if i+1 < len(line) && !isSpace(line[i+1]) {
						return nil, ErrUnbalancedQuote
					}
					inDQ = false
					i++
					args = append(args, cur)
					goto next
				default:
					cur = append(cur, c)
				}
				i++
				continue
			}
			if inSQ {
				if i >= len(line) {
					return nil, ErrUnbalancedQuote
				}
				c := line[i]
				switch {
				case c == '\\' && i+1 < len(line) && line[i+1] == '\'':
					cur = append(cur, '\'')
					i++
				case c == '\'':
					if i+1 < len(line) && !isSpace(line[i+1]) {
						return nil, ErrUnbalancedQuote
					}
					inSQ = false
					i++
					args = append(args, cur)
					goto next
				default:
					cur = append(cur, c)
				}
				i++
				continue
			}
			if i >= len(line) || isSpace(line[i]) {
				args = append(args, cur)
				goto next
			}
			switch line[i] {
			case '"':
				inDQ = true
			case '\'':
				inSQ = true
			default:
				cur = append(cur, line[i])
			}
			i++
		}
	next:
	}
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
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
	if len(line) > 0 && (line[0] == '+' || line[0] == '-' && string(line) != "-1") {
		return nil, ErrInvalidFormat
	}

	size, err := strconv.ParseInt(string(line), 10, 64)
	if err != nil {
		return nil, ErrInvalidFormat
	}

	if size == -1 {
		return NullBulkString(), nil
	}

	if size < 0 {
		return nil, ErrInvalidFormat
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
	if len(line) > 0 && (line[0] == '+' || line[0] == '-' && string(line) != "-1") {
		return nil, ErrInvalidFormat
	}

	count, err := strconv.ParseInt(string(line), 10, 64)
	if err != nil {
		return nil, ErrInvalidFormat
	}

	if count == -1 {
		return NullArray(), nil
	}

	if count < 0 {
		return nil, ErrInvalidFormat
	}

	if count == 0 {
		return ArrayValue([]*Value{}), nil
	}

	if count > MaxArrayElements {
		return nil, ErrArrayTooLarge
	}

	if r.depth >= MaxNestingDepth {
		return nil, ErrInvalidFormat
	}
	r.depth++
	defer func() { r.depth-- }()

	elements := make([]*Value, 0, min(count, 1024))
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
	line, err := r.rd.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	// Strip trailing \r\n
	if len(line) >= 2 && line[len(line)-2] == '\r' && bytes.IndexByte(line[:len(line)-2], '\r') < 0 {
		return line[:len(line)-2], nil
	}
	return nil, ErrInvalidFormat
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
