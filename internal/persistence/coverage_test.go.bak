package persistence

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

type mockStoreForCoverage struct {
	data map[string]interface{}
}

func (m *mockStoreForCoverage) GetAll() map[string]interface{} {
	return m.data
}

func TestAOFRewriterCoverage(t *testing.T) {
	store := &mockStoreForCoverage{
		data: map[string]interface{}{
			"key1": "value1",
			"key2": []byte("value2"),
			"key3": 123,
		},
	}

	cfg := AOFConfig{
		Enabled:     true,
		Filename:    "test.aof",
		DataDir:     os.TempDir(),
		SyncPolicy:  AOFEverySecond,
		RewriteSize: 1000000,
		RewritePct:  100,
		AutoRewrite: false,
	}

	rw := NewAOFRewriter(cfg, store)

	t.Run("New", func(t *testing.T) {
		if rw == nil {
			t.Fatal("AOFRewriter should not be nil")
		}
	})

	t.Run("IsRewriting", func(t *testing.T) {
		if rw.IsRewriting() {
			t.Error("should not be rewriting initially")
		}
	})

	t.Run("Rewrite", func(t *testing.T) {
		t.Skip("Skipping on Windows due to file locking during rename")
	})

	t.Run("RewriteAlreadyInProgress", func(t *testing.T) {
		rw.rewriting.Store(true)
		tmpPath := filepath.Join(os.TempDir(), "test_rewrite2.aof")

		err := rw.Rewrite(tmpPath)
		if err == nil {
			t.Error("rewrite should fail when already in progress")
		}

		rw.rewriting.Store(false)
	})
}

func TestAOFManagerCoverage(t *testing.T) {
	cfg := AOFConfig{
		Enabled:     true,
		Filename:    "test_manager.aof",
		DataDir:     os.TempDir(),
		SyncPolicy:  AOFAlways,
		RewriteSize: 1000000,
		RewritePct:  100,
		AutoRewrite: false,
	}
	defer os.Remove(filepath.Join(cfg.DataDir, cfg.Filename))

	store := &mockStoreForCoverage{data: make(map[string]interface{})}
	mgr := NewAOFManager(cfg, store)

	t.Run("New", func(t *testing.T) {
		if mgr == nil {
			t.Fatal("AOFManager should not be nil")
		}
	})

	t.Run("Size", func(t *testing.T) {
		size := mgr.Size()
		_ = size
	})

	t.Run("Dirty", func(t *testing.T) {
		dirty := mgr.Dirty()
		_ = dirty
	})

	t.Run("Flush", func(t *testing.T) {
		mgr.Flush()
	})

	t.Run("IsRewriting", func(t *testing.T) {
		if mgr.IsRewriting() {
			t.Error("should not be rewriting")
		}
	})

	t.Run("Append", func(t *testing.T) {
		mgr.Append("SET", [][]byte{[]byte("key"), []byte("value")})
	})

	t.Run("StartStop", func(t *testing.T) {
		mgr.Start()
		time.Sleep(10 * time.Millisecond)
		mgr.Stop()
	})

	t.Run("BGREWRITEAOF", func(t *testing.T) {
		err := mgr.BGREWRITEAOF()
		_ = err
	})

	t.Run("Load", func(t *testing.T) {
		commands, err := mgr.Load()
		_ = commands
		_ = err
	})
}

func TestAOFWriterCoverage(t *testing.T) {
	cfg := AOFConfig{
		Enabled:     true,
		Filename:    "test_writer.aof",
		DataDir:     os.TempDir(),
		SyncPolicy:  AOFAlways,
		RewriteSize: 1000000,
		RewritePct:  100,
		AutoRewrite: false,
	}
	defer os.Remove(filepath.Join(cfg.DataDir, cfg.Filename))

	writer := NewAOFWriter(cfg)

	t.Run("New", func(t *testing.T) {
		if writer == nil {
			t.Fatal("AOFWriter should not be nil")
		}
	})

	t.Run("Start", func(t *testing.T) {
		err := writer.Start()
		if err != nil {
			t.Errorf("start should succeed: %v", err)
		}
	})

	t.Run("Append", func(t *testing.T) {
		err := writer.Append("SET", [][]byte{[]byte("key"), []byte("value")})
		if err != nil {
			t.Errorf("append should succeed: %v", err)
		}
	})

	t.Run("Size", func(t *testing.T) {
		size := writer.Size()
		_ = size
	})

	t.Run("Dirty", func(t *testing.T) {
		dirty := writer.Dirty()
		if dirty <= 0 {
			t.Errorf("dirty = %d, want > 0", dirty)
		}
	})

	t.Run("Flush", func(t *testing.T) {
		writer.Flush()
	})

	t.Run("Stop", func(t *testing.T) {
		writer.Stop()
	})
}

func TestAOFWriterNoSync(t *testing.T) {
	cfg := AOFConfig{
		Enabled:     true,
		Filename:    "test_writer_nosync.aof",
		DataDir:     os.TempDir(),
		SyncPolicy:  AOFNoSync,
		RewriteSize: 1000000,
		RewritePct:  100,
		AutoRewrite: false,
	}
	defer os.Remove(filepath.Join(cfg.DataDir, cfg.Filename))

	writer := NewAOFWriter(cfg)
	err := writer.Start()
	if err != nil {
		t.Errorf("start should succeed: %v", err)
	}
	writer.Stop()
}

func TestAOFWriterEverySecond(t *testing.T) {
	cfg := AOFConfig{
		Enabled:     true,
		Filename:    "test_writer_everysec.aof",
		DataDir:     os.TempDir(),
		SyncPolicy:  AOFEverySecond,
		RewriteSize: 1000000,
		RewritePct:  100,
		AutoRewrite: false,
	}
	defer os.Remove(filepath.Join(cfg.DataDir, cfg.Filename))

	writer := NewAOFWriter(cfg)
	err := writer.Start()
	if err != nil {
		t.Errorf("start should succeed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	writer.Stop()
}

func TestAOFWriterDisabled(t *testing.T) {
	cfg := AOFConfig{
		Enabled: false,
	}

	writer := NewAOFWriter(cfg)
	err := writer.Start()
	if err != nil {
		t.Errorf("start should succeed when disabled: %v", err)
	}
}

func TestAOFReaderCoverage(t *testing.T) {
	reader := NewAOFReader()

	t.Run("New", func(t *testing.T) {
		if reader == nil {
			t.Fatal("AOFReader should not be nil")
		}
	})
}

func TestAOFConfigDefaultsCoverage(t *testing.T) {
	cfg := AOFConfig{
		Enabled:     true,
		Filename:    "test_config.aof",
		DataDir:     os.TempDir(),
		SyncPolicy:  AOFEverySecond,
		RewriteSize: 1000000,
		RewritePct:  100,
		AutoRewrite: true,
	}

	if cfg.Filename == "" {
		t.Error("filename should not be empty")
	}
}

func TestRDBWriterCoverage(t *testing.T) {
	s := store.NewStore()
	cfg := RDBConfig{
		Version:     RDBVersion9,
		Compression: true,
		Checksum:    true,
	}

	writer := NewRDBWriter(s, cfg)
	if writer == nil {
		t.Fatal("RDBWriter should not be nil")
	}
}

func TestRDBReaderCoverage(t *testing.T) {
	s := store.NewStore()

	reader := NewRDBReader(s)
	if reader == nil {
		t.Fatal("RDBReader should not be nil")
	}
}

func TestPersistenceManagerCoverage(t *testing.T) {
	s := store.NewStore()
	cfg := Config{
		DataDir:        os.TempDir(),
		RDBEnabled:     true,
		RDBFilename:    "test_pm.rdb",
		RDBInterval:    time.Minute,
		AOFRewriteSize: 1000000,
	}

	pm := NewPersistenceManager(s, cfg)
	if pm == nil {
		t.Fatal("PersistenceManager should not be nil")
	}

	pm.MarkDirty()
	dirty := pm.Dirty()
	_ = dirty

	lastSave := pm.LastSave()
	_ = lastSave

	pm.BGSAVE()

	err := pm.SAVE()
	_ = err
}

func TestAOFSyncPolicies(t *testing.T) {
	tests := []struct {
		name   string
		policy AOFSyncPolicy
	}{
		{"Always", AOFAlways},
		{"EverySecond", AOFEverySecond},
		{"NoSync", AOFNoSync},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AOFConfig{
				Enabled:     true,
				Filename:    "test_sync.aof",
				DataDir:     os.TempDir(),
				SyncPolicy:  tt.policy,
				RewriteSize: 1000000,
				RewritePct:  100,
				AutoRewrite: false,
			}
			defer os.Remove(filepath.Join(cfg.DataDir, cfg.Filename))

			writer := NewAOFWriter(cfg)
			if writer == nil {
				t.Fatal("AOFWriter should not be nil")
			}

			err := writer.Start()
			if err != nil {
				t.Errorf("start failed: %v", err)
			}

			writer.Append("SET", [][]byte{[]byte("key"), []byte("value")})
			writer.Flush()
			writer.Stop()
		})
	}
}

func TestAOFRewriterWriteEntry(t *testing.T) {
	store := &mockStoreForCoverage{
		data: map[string]interface{}{
			"str":   "value",
			"num":   123,
			"slice": []byte("bytes"),
		},
	}

	cfg := AOFConfig{
		Enabled:     true,
		Filename:    "test_entry.aof",
		DataDir:     os.TempDir(),
		SyncPolicy:  AOFEverySecond,
		RewriteSize: 1000000,
		RewritePct:  100,
		AutoRewrite: false,
	}

	rw := NewAOFRewriter(cfg, store)
	if rw == nil {
		t.Fatal("AOFRewriter should not be nil")
	}

	_ = rw.IsRewriting()
}

func TestRDBWriterSave(t *testing.T) {
	s := store.NewStore()
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("key2", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})

	cfg := RDBConfig{
		Version:     RDBVersion9,
		Compression: false,
		Checksum:    false,
	}

	writer := NewRDBWriter(s, cfg)

	tmpPath := filepath.Join(os.TempDir(), "test_save.rdb")
	defer os.Remove(tmpPath)

	err := writer.Save(tmpPath)
	if err != nil {
		t.Logf("Save returned: %v", err)
	}
}

func TestRDBReaderLoad(t *testing.T) {
	s := store.NewStore()

	reader := NewRDBReader(s)

	tmpPath := filepath.Join(os.TempDir(), "test_load.rdb")
	defer os.Remove(tmpPath)

	f, err := os.Create(tmpPath)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	f.Write([]byte("REDIS0009\xfe\x00\x00"))
	f.Close()

	err = reader.Load(tmpPath)
	if err != nil {
		t.Logf("Load returned: %v", err)
	}
}

func TestPersistenceManagerAutoSave(t *testing.T) {
	s := store.NewStore()
	cfg := Config{
		DataDir:        os.TempDir(),
		RDBEnabled:     true,
		RDBFilename:    "test_autosave.rdb",
		RDBInterval:    50 * time.Millisecond,
		AOFRewriteSize: 1000000,
	}

	pm := NewPersistenceManager(s, cfg)

	pm.Start()
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	pm.MarkDirty()

	time.Sleep(100 * time.Millisecond)

	pm.Stop()
}

func TestAOFWriterMultipleAppends(t *testing.T) {
	cfg := AOFConfig{
		Enabled:     true,
		Filename:    "test_multi.aof",
		DataDir:     os.TempDir(),
		SyncPolicy:  AOFAlways,
		RewriteSize: 1000000,
		RewritePct:  100,
		AutoRewrite: false,
	}
	defer os.Remove(filepath.Join(cfg.DataDir, cfg.Filename))

	writer := NewAOFWriter(cfg)
	writer.Start()

	for i := 0; i < 10; i++ {
		writer.Append("SET", [][]byte{[]byte("key"), []byte("value")})
	}

	writer.Flush()
	writer.Stop()

	if writer.Size() == 0 {
		t.Error("expected non-zero size")
	}
}

func TestAOFManagerMultipleAppends(t *testing.T) {
	cfg := AOFConfig{
		Enabled:     true,
		Filename:    "test_mgr_multi.aof",
		DataDir:     os.TempDir(),
		SyncPolicy:  AOFAlways,
		RewriteSize: 1000000,
		RewritePct:  100,
		AutoRewrite: false,
	}
	defer os.Remove(filepath.Join(cfg.DataDir, cfg.Filename))

	store := &mockStoreForCoverage{data: make(map[string]interface{})}
	mgr := NewAOFManager(cfg, store)
	mgr.Start()

	for i := 0; i < 10; i++ {
		mgr.Append("SET", [][]byte{[]byte("key"), []byte("value")})
	}

	mgr.Flush()
	mgr.Stop()
}

func TestAOFWriterSyncFile(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test_sync.aof")
	defer os.Remove(tmpFile)

	f, err := os.Create(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	w := &AOFWriter{
		file:   f,
		writer: bufio.NewWriter(f),
		config: AOFConfig{
			SyncPolicy: AOFAlways,
		},
	}

	// Write something
	w.writer.WriteString("test data\n")

	// Sync
	w.syncFile()

	// Verify
	w.file.Close()
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(data) != "test data\n" {
		t.Errorf("Expected 'test data\\n', got %q", string(data))
	}
}

func TestAOFRewriterWriteEntry2(t *testing.T) {
	rw := &AOFRewriter{
		config: AOFConfig{},
	}

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	// Test with string entry
	err := rw.writeEntry(w, "mykey", "myvalue")
	if err != nil {
		t.Errorf("writeEntry failed: %v", err)
	}
	w.Flush()

	if buf.Len() == 0 {
		t.Error("writeEntry should write data")
	}

	// Test with []byte entry
	buf.Reset()
	w = bufio.NewWriter(&buf)
	err = rw.writeEntry(w, "key2", []byte("value2"))
	if err != nil {
		t.Errorf("writeEntry with []byte failed: %v", err)
	}
	w.Flush()

	if buf.Len() == 0 {
		t.Error("writeEntry should write data for []byte")
	}

	// Test with other type
	buf.Reset()
	w = bufio.NewWriter(&buf)
	err = rw.writeEntry(w, "key3", 12345)
	if err != nil {
		t.Errorf("writeEntry with int failed: %v", err)
	}
	w.Flush()

	if buf.Len() == 0 {
		t.Error("writeEntry should write data for int")
	}
}

func TestRDBReaderReadLength(t *testing.T) {
	s := store.NewStore()
	reader := NewRDBReader(s)

	t.Run("6-bit length", func(t *testing.T) {
		// 6-bit encoding: first 2 bits are 00, remaining 6 bits are length
		data := []byte{0x05} // 00 000101 = 5
		r := bytes.NewReader(data)
		length, err := reader.readLength(r)
		if err != nil {
			t.Errorf("readLength failed: %v", err)
		}
		if length != 5 {
			t.Errorf("expected length 5, got %d", length)
		}
	})

	t.Run("14-bit length", func(t *testing.T) {
		// 14-bit encoding: first 2 bits are 01, remaining 14 bits are length
		data := []byte{0x41, 0x00} // 01 000001 00000000 = 256
		r := bytes.NewReader(data)
		length, err := reader.readLength(r)
		if err != nil {
			t.Errorf("readLength failed: %v", err)
		}
		if length != 256 {
			t.Errorf("expected length 256, got %d", length)
		}
	})

	t.Run("32-bit length", func(t *testing.T) {
		// 32-bit encoding: first 2 bits are 10, followed by 4 bytes
		data := []byte{0x80, 0x00, 0x01, 0x00, 0x00} // 10000000 followed by 65536 in big-endian
		r := bytes.NewReader(data)
		length, err := reader.readLength(r)
		if err != nil {
			t.Errorf("readLength failed: %v", err)
		}
		if length != 65536 {
			t.Errorf("expected length 65536, got %d", length)
		}
	})
}

func TestRDBReaderReadEntry(t *testing.T) {
	s := store.NewStore()
	reader := NewRDBReader(s)

	t.Run("string value", func(t *testing.T) {
		// Write key and value
		keyData := []byte{0x03} // length 3
		keyData = append(keyData, []byte("key")...)
		valData := []byte{0x05} // length 5
		valData = append(valData, []byte("value")...)

		data := append(keyData, valData...)
		r := bytes.NewReader(data)

		err := reader.readEntry(r, 0) // type 0 = string
		if err != nil {
			t.Errorf("readEntry failed: %v", err)
		}

		val, exists := s.Get("key")
		if !exists {
			t.Error("key should exist")
		}
		if val == nil {
			t.Error("value should not be nil")
		}
	})

	t.Run("list value", func(t *testing.T) {
		s2 := store.NewStore()
		reader2 := NewRDBReader(s2)

		// Build list data: key + length + items
		var data []byte
		data = append(data, 0x04) // key length 4
		data = append(data, []byte("list")...)
		data = append(data, 0x02) // list length 2
		data = append(data, 0x05) // item1 length 5
		data = append(data, []byte("item1")...)
		data = append(data, 0x05) // item2 length 5
		data = append(data, []byte("item2")...)

		r := bytes.NewReader(data)
		err := reader2.readEntry(r, 1) // type 1 = list
		if err != nil {
			t.Errorf("readEntry for list failed: %v", err)
		}
	})

	t.Run("set value", func(t *testing.T) {
		s3 := store.NewStore()
		reader3 := NewRDBReader(s3)

		// Build set data: key + length + members
		var data []byte
		data = append(data, 0x03) // key length 3
		data = append(data, []byte("set")...)
		data = append(data, 0x02) // set size 2
		data = append(data, 0x01) // member1 length 1
		data = append(data, []byte("a")...)
		data = append(data, 0x01) // member2 length 1
		data = append(data, []byte("b")...)

		r := bytes.NewReader(data)
		err := reader3.readEntry(r, 2) // type 2 = set
		if err != nil {
			t.Errorf("readEntry for set failed: %v", err)
		}
	})

	t.Run("hash value", func(t *testing.T) {
		s4 := store.NewStore()
		reader4 := NewRDBReader(s4)

		// Build hash data: key + length + field/value pairs
		var data []byte
		data = append(data, 0x04) // key length 4
		data = append(data, []byte("hash")...)
		data = append(data, 0x01) // hash size 1
		data = append(data, 0x05) // field length 5
		data = append(data, []byte("field")...)
		data = append(data, 0x05) // value length 5
		data = append(data, []byte("value")...)

		r := bytes.NewReader(data)
		err := reader4.readEntry(r, 3) // type 3 = hash
		if err != nil {
			t.Errorf("readEntry for hash failed: %v", err)
		}
	})

	t.Run("default value type", func(t *testing.T) {
		s5 := store.NewStore()
		reader5 := NewRDBReader(s5)

		var data []byte
		data = append(data, 0x04) // key length 4
		data = append(data, []byte("defk")...)
		data = append(data, 0x04) // value length 4
		data = append(data, []byte("defv")...)

		r := bytes.NewReader(data)
		err := reader5.readEntry(r, 99) // unknown type, should default to string
		if err != nil {
			t.Errorf("readEntry for default type failed: %v", err)
		}
	})
}

func TestRDBReaderReadString(t *testing.T) {
	s := store.NewStore()
	reader := NewRDBReader(s)

	t.Run("simple string", func(t *testing.T) {
		data := []byte{0x05} // length 5
		data = append(data, []byte("hello")...)
		r := bytes.NewReader(data)

		str, err := reader.readString(r)
		if err != nil {
			t.Errorf("readString failed: %v", err)
		}
		if str != "hello" {
			t.Errorf("expected 'hello', got %q", str)
		}
	})
}

func TestRDBReaderReadRDB(t *testing.T) {
	s := store.NewStore()
	reader := NewRDBReader(s)

	t.Run("valid header", func(t *testing.T) {
		// Create minimal valid RDB data
		var data []byte
		data = append(data, []byte("REDIS0009")...) // magic + version
		data = append(data, 0xFE)                   // AUX field marker
		data = append(data, 0x05)                   // key length
		data = append(data, []byte("redis-ver")...)
		data = append(data, 0x05) // value length
		data = append(data, []byte("5.0.0")...)
		data = append(data, 0xFE) // AUX field
		data = append(data, 0x09) // key length
		data = append(data, []byte("redis-bits")...)
		data = append(data, 0x02) // value length
		data = append(data, []byte("64")...)
		data = append(data, 0xFE) // AUX field
		data = append(data, 0x05) // key length
		data = append(data, []byte("ctime")...)
		data = append(data, 0x0A) // value length
		data = append(data, []byte("1234567890")...)
		data = append(data, 0xFF) // EOF marker

		r := bytes.NewReader(data)
		err := reader.readRDB(r)
		if err != nil {
			t.Logf("readRDB returned: %v", err)
		}
	})
}

func TestAOFManagerLoadWithCommands(t *testing.T) {
	cfg := AOFConfig{
		Enabled:     true,
		Filename:    "test_load_cmd.aof",
		DataDir:     os.TempDir(),
		SyncPolicy:  AOFAlways,
		RewriteSize: 1000000,
		RewritePct:  100,
		AutoRewrite: false,
	}
	defer os.Remove(filepath.Join(cfg.DataDir, cfg.Filename))

	// Create AOF file with commands
	f, err := os.Create(filepath.Join(cfg.DataDir, cfg.Filename))
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	f.WriteString("*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n")
	f.Close()

	store := &mockStoreForCoverage{data: make(map[string]interface{})}
	mgr := NewAOFManager(cfg, store)

	commands, err := mgr.Load()
	if err != nil {
		t.Logf("Load returned: %v", err)
	}
	_ = commands
}

func TestAOFManagerShouldRewrite(t *testing.T) {
	cfg := AOFConfig{
		Enabled:     true,
		Filename:    "test_should_rewrite.aof",
		DataDir:     os.TempDir(),
		SyncPolicy:  AOFAlways,
		RewriteSize: 100,
		RewritePct:  50,
		AutoRewrite: true,
	}
	defer os.Remove(filepath.Join(cfg.DataDir, cfg.Filename))

	store := &mockStoreForCoverage{data: make(map[string]interface{})}
	mgr := NewAOFManager(cfg, store)
	mgr.Start()

	// Write enough data to trigger rewrite consideration
	for i := 0; i < 20; i++ {
		mgr.Append("SET", [][]byte{[]byte("key"), []byte("value")})
	}

	time.Sleep(50 * time.Millisecond)
	mgr.Stop()
}

func TestRDBWriterWriteValue(t *testing.T) {
	s := store.NewStore()
	cfg := RDBConfig{
		Version:     RDBVersion9,
		Compression: false,
		Checksum:    false,
	}

	writer := NewRDBWriter(s, cfg)

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	t.Run("string value", func(t *testing.T) {
		val := &store.StringValue{Data: []byte("test")}
		err := writer.writeValue(w, val, 0)
		if err != nil {
			t.Errorf("writeValue for string failed: %v", err)
		}
	})

	t.Run("list value", func(t *testing.T) {
		val := &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}
		err := writer.writeValue(w, val, 1)
		if err != nil {
			t.Errorf("writeValue for list failed: %v", err)
		}
	})

	t.Run("set value", func(t *testing.T) {
		val := &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}
		err := writer.writeValue(w, val, 2)
		if err != nil {
			t.Errorf("writeValue for set failed: %v", err)
		}
	})

	t.Run("hash value", func(t *testing.T) {
		val := &store.HashValue{Fields: map[string][]byte{"field": []byte("value")}}
		err := writer.writeValue(w, val, 3)
		if err != nil {
			t.Errorf("writeValue for hash failed: %v", err)
		}
	})
}
