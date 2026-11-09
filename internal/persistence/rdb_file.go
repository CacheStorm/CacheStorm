package persistence

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"os"
	"path/filepath"
	"runtime"
)

// rdbCRCTables are Redis' CRC64 (Jones polynomial, reflected, no final
// xor) tables for slicing by 8: rdbCRCTables[0] is the byte-wise table and
// rdbCRCTables[k] advances a byte k more positions.
var rdbCRCTables = func() *[8][256]uint64 {
	var t [8][256]uint64
	poly := bits.Reverse64(0xad93d23594c935a9)
	for i := range t[0] {
		v := uint64(i)
		for j := 0; j < 8; j++ {
			if v&1 != 0 {
				v = v>>1 ^ poly
			} else {
				v >>= 1
			}
		}
		t[0][i] = v
	}
	for i := range t[0] {
		v := t[0][i]
		for k := 1; k < 8; k++ {
			v = t[0][byte(v)] ^ v>>8
			t[k][i] = v
		}
	}
	return &t
}()

func rdbChecksum(crc uint64, data []byte) uint64 {
	t := rdbCRCTables
	for len(data) >= 8 {
		crc ^= binary.LittleEndian.Uint64(data)
		crc = t[7][byte(crc)] ^ t[6][byte(crc>>8)] ^ t[5][byte(crc>>16)] ^ t[4][byte(crc>>24)] ^
			t[3][byte(crc>>32)] ^ t[2][byte(crc>>40)] ^ t[1][byte(crc>>48)] ^ t[0][crc>>56]
		data = data[8:]
	}
	for _, b := range data {
		crc = t[0][byte(crc)^b] ^ crc>>8
	}
	return crc
}

type rdbChecksumWriter struct {
	io.Writer
	checksum uint64
}

func (w *rdbChecksumWriter) Write(data []byte) (int, error) {
	n, err := w.Writer.Write(data)
	w.checksum = rdbChecksum(w.checksum, data[:n])
	return n, err
}

func WriteRDBFile(path string, data []byte) error {
	return writeRDBAtomic(path, func(w io.Writer) error {
		n, err := w.Write(data)
		if err == nil && n != len(data) {
			err = io.ErrShortWrite
		}
		return err
	})
}

func writeRDBAtomic(path string, write func(io.Writer) error) error {
	return writeAtomic(path, ".cachestorm-rdb-*", write)
}

// WriteFileAtomic replaces path with data: it writes a temporary file in the
// same directory, syncs it, renames it over path and syncs the directory.
func WriteFileAtomic(path string, data []byte) error {
	return writeAtomic(path, ".cachestorm-tmp-*", func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

func writeAtomic(path, pattern string, write func(io.Writer) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := write(f); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}

// SealRDBChecksum fills the 8-byte CRC64 trailer of a snapshot encoded with
// Checksum disabled, so the checksum can be computed after the encoder has
// released the dataset.
func SealRDBChecksum(data []byte) error {
	if len(data) < 9 || data[len(data)-9] != 0xFF {
		return errors.New("snapshot does not end with an RDB trailer")
	}
	binary.LittleEndian.PutUint64(data[len(data)-8:], rdbChecksum(0, data[:len(data)-8]))
	return nil
}
