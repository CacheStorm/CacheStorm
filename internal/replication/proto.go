package replication

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// Limits applied to what a peer sends over a replication link.
const (
	maxLineLen  = 64 * 1024
	maxArgs     = 1024 * 1024
	maxBulkLen  = 512 * 1024 * 1024
)

var errProtocol = errors.New("replication protocol error")

// appendCommand appends the RESP multi-bulk encoding of name and args.
func appendCommand(buf []byte, name string, args [][]byte) []byte {
	buf = append(buf, '*')
	buf = strconv.AppendInt(buf, int64(len(args)+1), 10)
	buf = append(buf, '\r', '\n', '$')
	buf = strconv.AppendInt(buf, int64(len(name)), 10)
	buf = append(buf, '\r', '\n')
	buf = append(buf, name...)
	buf = append(buf, '\r', '\n')
	for _, a := range args {
		buf = append(buf, '$')
		buf = strconv.AppendInt(buf, int64(len(a)), 10)
		buf = append(buf, '\r', '\n')
		buf = append(buf, a...)
		buf = append(buf, '\r', '\n')
	}
	return buf
}

// encodeCommand returns the RESP encoding of a command given as strings.
func encodeCommand(parts ...string) []byte {
	args := make([][]byte, len(parts)-1)
	for i, p := range parts[1:] {
		args[i] = []byte(p)
	}
	return appendCommand(nil, parts[0], args)
}

// readLine reads one CRLF- or LF-terminated line without its terminator.
func readLine(br *bufio.Reader) (string, error) {
	var sb strings.Builder
	for {
		chunk, err := br.ReadSlice('\n')
		sb.Write(chunk)
		if err == nil {
			break
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return "", err
		}
		if sb.Len() > maxLineLen {
			return "", fmt.Errorf("%w: line too long", errProtocol)
		}
	}
	line := sb.String()
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}

// readCommand reads one multi-bulk command and returns its arguments (the
// name first) and the exact bytes it occupied in the stream.
func readCommand(br *bufio.Reader) ([][]byte, []byte, error) {
	header, err := readLine(br)
	if err != nil {
		return nil, nil, err
	}
	if len(header) < 2 || header[0] != '*' {
		return nil, nil, fmt.Errorf("%w: expected a multi-bulk command, got %q", errProtocol, truncate(header))
	}
	n, err := strconv.Atoi(header[1:])
	if err != nil || n < 1 || n > maxArgs {
		return nil, nil, fmt.Errorf("%w: bad argument count %q", errProtocol, truncate(header))
	}
	raw := make([]byte, 0, len(header)+2+n*16)
	raw = append(raw, header...)
	raw = append(raw, '\r', '\n')
	args := make([][]byte, n)
	for i := 0; i < n; i++ {
		line, err := readLine(br)
		if err != nil {
			return nil, nil, err
		}
		if len(line) < 2 || line[0] != '$' {
			return nil, nil, fmt.Errorf("%w: expected a bulk string, got %q", errProtocol, truncate(line))
		}
		size, err := strconv.Atoi(line[1:])
		if err != nil || size < 0 || size > maxBulkLen {
			return nil, nil, fmt.Errorf("%w: bad bulk length %q", errProtocol, truncate(line))
		}
		data := make([]byte, size+2)
		if _, err := io.ReadFull(br, data); err != nil {
			return nil, nil, err
		}
		if data[size] != '\r' || data[size+1] != '\n' {
			return nil, nil, fmt.Errorf("%w: bulk string not terminated by CRLF", errProtocol)
		}
		args[i] = data[:size:size]
		raw = append(raw, line...)
		raw = append(raw, '\r', '\n')
		raw = append(raw, data...)
	}
	return args, raw, nil
}

func truncate(s string) string {
	if len(s) > 64 {
		return s[:64] + "..."
	}
	return s
}

// deadlineReader re-arms the connection's read deadline before every read, so
// a link is dropped only after timeout of silence, however long the whole
// transfer takes.
type deadlineReader struct {
	conn    net.Conn
	timeout time.Duration
}

func (d *deadlineReader) Read(p []byte) (int, error) {
	if d.timeout > 0 {
		_ = d.conn.SetReadDeadline(time.Now().Add(d.timeout))
	}
	return d.conn.Read(p)
}

// parseFullResync parses "+FULLRESYNC <replid> <offset>".
func parseFullResync(line string) (string, int64, error) {
	fields := strings.Fields(strings.TrimPrefix(line, "+FULLRESYNC"))
	if len(fields) != 2 {
		return "", 0, fmt.Errorf("%w: malformed FULLRESYNC reply %q", errProtocol, truncate(line))
	}
	off, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || off < 0 {
		return "", 0, fmt.Errorf("%w: malformed FULLRESYNC offset %q", errProtocol, truncate(line))
	}
	return fields[0], off, nil
}

// parseContinue parses "+CONTINUE [<replid>]"; the id is empty when the
// master did not send one.
func parseContinue(line string) string {
	fields := strings.Fields(strings.TrimPrefix(line, "+CONTINUE"))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// parseBulkHeader parses the "$<len>" line that precedes a snapshot.
func parseBulkHeader(line string) (int64, error) {
	if len(line) < 2 || line[0] != '$' {
		return 0, fmt.Errorf("%w: expected the snapshot length, got %q", errProtocol, truncate(line))
	}
	n, err := strconv.ParseInt(line[1:], 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: bad snapshot length %q", errProtocol, truncate(line))
	}
	return n, nil
}
