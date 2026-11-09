package persistence

import (
	"bytes"
	"errors"
	"io"
)

const AOFVersion = 2

// aofVersionCommand is the header of AOFs CacheStorm 0.3 and earlier wrote:
// a command, which Redis refuses as unknown.
const aofVersionCommand = "CACHESTORM.AOF"

// aofVersionLine is the header of the AOFs CacheStorm writes now. It is an
// annotation, a line starting with '#', which Redis 7 skips as it skips its
// own "#TS:" lines, so Redis can load an AOF CacheStorm wrote.
const aofVersionLine = "#CACHESTORM.AOF 2\r\n"

// aofVersionPrefix starts a version annotation of any version.
const aofVersionPrefix = "#CACHESTORM.AOF "

var errAOFVersion = errors.New("invalid or unsupported AOF version header")

func writeAOFHeader(w io.Writer) error {
	n, err := io.WriteString(w, aofVersionLine)
	if err == nil && n != len(aofVersionLine) {
		return io.ErrShortWrite
	}
	return err
}

// aofVersionAnnotation reports whether line, an annotation read at offset
// valid, is a version header, and whether it is one this version reads.
func aofVersionAnnotation(line []byte, valid int64, version int) (isHeader bool, err error) {
	if !bytes.HasPrefix(line, []byte(aofVersionPrefix)) {
		return false, nil
	}
	if valid != 0 || version != 0 || string(bytes.TrimRight(line, "\r\n")) != aofVersionLine[:len(aofVersionLine)-2] {
		return true, errAOFVersion
	}
	return true, nil
}
