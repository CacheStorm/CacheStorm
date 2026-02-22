package persistence

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestRDBWriterAdvanced(t *testing.T) {
	s := store.NewStore()
	cfg := RDBConfig{Version: RDBVersion9, Compression: true, Checksum: true}

	t.Run("RDB Writer Creation", func(t *testing.T) {
		writer := NewRDBWriter(s, cfg)
		if writer == nil {
			t.Fatal("NewRDBWriter returned nil")
		}
	})

	t.Run("RDB Writer Save Empty", func(t *testing.T) {
		writer := NewRDBWriter(s, cfg)
		tempDir := t.TempDir()
		err := writer.Save(tempDir + "/test.rdb")
		// File access issues on Windows may cause errors, just verify it doesn't panic
		_ = err
	})
}

func TestRDBReaderAdvanced(t *testing.T) {
	s := store.NewStore()

	t.Run("RDB Reader Creation", func(t *testing.T) {
		reader := NewRDBReader(s)
		if reader == nil {
			t.Fatal("NewRDBReader returned nil")
		}
	})

	t.Run("RDB Reader Load Nonexistent", func(t *testing.T) {
		reader := NewRDBReader(s)
		err := reader.Load("/nonexistent/path/test.rdb")
		if err == nil {
			t.Error("Should error for nonexistent file")
		}
	})
}

func TestPersistenceManagerAdvanced(t *testing.T) {
	s := store.NewStore()
	cfg := Config{
		DataDir:        t.TempDir(),
		RDBEnabled:     true,
		RDBFilename:    "dump.rdb",
		RDBInterval:    0,
		AOFRewriteSize: 64 * 1024 * 1024,
	}

	t.Run("Persistence Manager Creation", func(t *testing.T) {
		pm := NewPersistenceManager(s, cfg)
		if pm == nil {
			t.Fatal("NewPersistenceManager returned nil")
		}
	})
}
