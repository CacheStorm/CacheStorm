package persistence

import (
	"io"
)

const AOFVersion = 2
const aofVersionCommand = "CACHESTORM.AOF"

func writeAOFHeader(w io.Writer) error {
	data := appendCommand(nil, aofVersionCommand, [][]byte{[]byte("2")})
	n, err := w.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}
