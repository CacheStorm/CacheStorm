package command

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestBitmapGrowthRejectsMemoryBeforeMutation(t *testing.T) {
	for _, name := range []string{"SETBIT", "BITFIELD"} {
		t.Run(name, func(t *testing.T) {
			s := store.NewStore()
			s.ConfigureMemory(4096, store.EvictionNoEviction, 70, 90, 5)
			deadline := time.Now().Add(time.Minute).UnixMilli()
			if err := s.Set("k", &BitmapValue{Data: []byte{128}}, store.SetOptions{ExpireAtMS: deadline, Tags: []string{"tag"}}); err != nil {
				t.Fatal(err)
			}
			before := s.MemoryTracker().Usage()
			version := s.WatchKey("k")
			defer s.UnwatchKey("k")
			r := NewRouter()
			RegisterBitmapCommands(r)
			var result string
			if name == "SETBIT" {
				result = execBf(s, r, name, "k", "65535", "1")
			} else {
				result = execBf(s, r, name, "k", "SET", "u8", "0", "7", "SET", "u8", "65528", "1")
			}
			if !strings.HasPrefix(result, "-OOM") {
				t.Fatalf("growth accepted beyond memory limit: %q", result)
			}
			got, ok := s.Get("k")
			if !ok || got.ExpiresAt.Load() != deadline || len(got.TagList()) != 1 || got.TagList()[0] != "tag" || !bytes.Equal([]byte(got.Value.String()), []byte{128}) {
				t.Fatal("rejected growth changed data or metadata")
			}
			if s.MemoryTracker().Usage() != before || s.MemUsage() != before {
				t.Fatal("rejected growth changed memory accounting")
			}
			if s.GetVersion("k") != version {
				t.Fatal("rejected growth changed watch version")
			}
		})
	}
}

func TestBitfieldReadAndInvalidSuffixDoNotCreateOrModify(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterBitmapCommands(r)
	if got := execBf(s, r, "BITFIELD", "missing", "GET", "u8", "100"); got != "*1\r\n:0\r\n" {
		t.Fatalf("read=%q", got)
	}
	if s.Exists("missing") {
		t.Fatal("read-only bitfield created missing key")
	}
	if err := s.Set("k", &store.StringValue{Data: []byte("original")}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	got := execBf(s, r, "BITFIELD", "k", "SET", "u8", "0", "7", "INVALID")
	if !strings.HasPrefix(got, "-ERR") {
		t.Fatalf("invalid suffix accepted: %q", got)
	}
	value, _ := s.Get("k")
	if value.Value.String() != "original" {
		t.Fatal("invalid suffix applied earlier write")
	}
}

func FuzzBoundedBitfieldOperations(f *testing.F) {
	for _, tc := range [][4]string{{"SET", "u8", "0", "1"}, {"SET", "i64", "4294967295", "1"}, {"GET", "i64", "4294967295", ""}, {"SET", "u64", "0", "1"}, {"INCRBY", "i64", "#9223372036854775807", "1"}, {"INCRBY", "i64", "0", "9223372036854775807"}} {
		f.Add(tc[0], tc[1], tc[2], tc[3])
	}
	f.Fuzz(func(t *testing.T, operation, encoding, offset, value string) {
		if len(operation)+len(encoding)+len(offset)+len(value) > 256 {
			t.Skip()
		}
		s := store.NewStore()
		s.ConfigureMemory(4096, store.EvictionNoEviction, 70, 90, 5)
		if err := s.Set("k", &BitmapValue{Data: []byte{128}}, store.SetOptions{}); err != nil {
			t.Fatal(err)
		}
		r := NewRouter()
		RegisterBitmapCommands(r)
		if operation == "GET" {
			execBf(s, r, "BITFIELD", "k", operation, encoding, offset)
		} else {
			execBf(s, r, "BITFIELD", "k", operation, encoding, offset, value)
		}
		if s.MemoryTracker().Usage() != s.MemUsage() || s.MemUsage() > 4096 {
			t.Fatal("operation escaped memory accounting or limit")
		}
	})
}

func TestBitmapMaximumOffsetsRespectMemoryAdmission(t *testing.T) {
	s := store.NewStore()
	s.ConfigureMemory(4096, store.EvictionNoEviction, 70, 90, 5)
	if err := s.Set("k", &BitmapValue{Data: []byte{128}}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	r := NewRouter()
	RegisterBitmapCommands(r)
	for _, args := range [][]string{{"SETBIT", "k", "4294967295", "1"}, {"BITFIELD", "k", "SET", "i64", "4294967295", "1"}} {
		if got := execBf(s, r, args[0], args[1:]...); !strings.HasPrefix(got, "-OOM") {
			t.Fatalf("maximum offset accepted: %q", got)
		}
	}
	got, _ := s.Get("k")
	if got.Value.String() != string([]byte{128}) || s.MemoryTracker().Usage() != s.MemUsage() {
		t.Fatal("maximum offset changed data/accounting")
	}
}
