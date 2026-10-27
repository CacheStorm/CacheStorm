package persistence

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAOFVersionHeaderValidation(t *testing.T) {
	var header bytes.Buffer
	if err := writeAOFHeader(&header); err != nil {
		t.Fatal(err)
	}
	set := appendCommand(nil, "SET", [][]byte{[]byte("k"), []byte("v")})
	for _, tc := range []struct {
		name    string
		data    []byte
		version int
		invalid bool
	}{
		{"legacy", set, 0, false},
		{"versioned", append(append([]byte{}, header.Bytes()...), set...), AOFVersion, false},
		{"unsupported", appendCommand(nil, aofVersionCommand, [][]byte{[]byte("3")}), 0, true},
		{"duplicate", append(append([]byte{}, header.Bytes()...), header.Bytes()...), 0, true},
		{"misplaced", append(append([]byte{}, set...), header.Bytes()...), 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "appendonly.aof")
			if err := os.WriteFile(path, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			reader := NewAOFReader()
			commands, valid, err := reader.LoadValid(path)
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid header accepted")
				}
				return
			}
			if err != nil || reader.Version != tc.version || valid != int64(len(tc.data)) || len(commands) != 1 {
				t.Fatalf("commands=%v version=%d valid=%d err=%v", commands, reader.Version, valid, err)
			}
		})
	}
}
