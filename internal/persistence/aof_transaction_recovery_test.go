package persistence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAOFValidPrefixExcludesEveryTornTransactionTail(t *testing.T) {
	prefix := []byte("*3\r\n$3\r\nSET\r\n$6\r\nstable\r\n$2\r\nok\r\n")
	block := []byte("*1\r\n$5\r\nMULTI\r\n*2\r\n$6\r\nSELECT\r\n$1\r\n3\r\n*3\r\n$3\r\nSET\r\n$1\r\nx\r\n$1\r\ny\r\n*1\r\n$4\r\nEXEC\r\n")
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	for cut := 0; cut <= len(block); cut++ {
		data := append(append([]byte(nil), prefix...), block[:cut]...)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		cmds, valid, err := NewAOFReader().LoadValid(path)
		if err != nil {
			t.Fatalf("cut %d: %v", cut, err)
		}
		wantValid, wantCommands := int64(len(prefix)), 1
		if cut == len(block) {
			wantValid, wantCommands = int64(len(data)), 5
		}
		if valid != wantValid || len(cmds) != wantCommands {
			t.Fatalf("cut %d: valid %d commands %d; want %d and %d", cut, valid, len(cmds), wantValid, wantCommands)
		}
	}
}

func TestAOFReaderRejectsInvalidTransactionFraming(t *testing.T) {
	for _, data := range []string{
		"*1\r\n$4\r\nEXEC\r\n",
		"*1\r\n$7\r\nDISCARD\r\n",
		"*1\r\n$5\r\nMULTI\r\n*1\r\n$5\r\nMULTI\r\n",
		"*2\r\n$5\r\nMULTI\r\n$1\r\nx\r\n",
		"*1\r\n$5\r\nMULTI\r\n*2\r\n$4\r\nEXEC\r\n$1\r\nx\r\n",
	} {
		path := filepath.Join(t.TempDir(), "appendonly.aof")
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := NewAOFReader().LoadValid(path); err == nil {
			t.Fatalf("invalid framing accepted: %q", data)
		}
	}
}
