package command

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestBitmapDumpAndStringReadersRaceWithWriters(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterKeyCommands(router)
	RegisterStringCommands(router)
	RegisterBitmapCommands(router)
	dumpRaceRun(t, s, router, "SETBIT", "bitmap", "32767", "1")
	var wg sync.WaitGroup
	for worker := 0; worker < 3; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				switch worker {
				case 0:
					dumpRaceRun(t, s, router, "SETBIT", "bitmap", strconv.Itoa(i%32768), strconv.Itoa(i%2))
					dumpRaceRun(t, s, router, "BITFIELD", "bitmap", "INCRBY", "u8", "0", "1")
				case 1:
					dumpRaceRun(t, s, router, "DUMP", "bitmap")
					dumpRaceRun(t, s, router, "GET", "bitmap")
				case 2:
					dumpRaceRun(t, s, router, "GETBIT", "bitmap", "0")
					dumpRaceRun(t, s, router, "BITCOUNT", "bitmap")
					dumpRaceRun(t, s, router, "BITPOS", "bitmap", "1")
					dumpRaceRun(t, s, router, "BITOP", "OR", "copy", "bitmap")
				}
			}
		}(worker)
	}
	wg.Wait()
}

func TestBitmapCreationConversionAndBitopReportStoreFailure(t *testing.T) {
	for _, name := range []string{"SETBIT", "BITFIELD", "BITOP"} {
		for _, existing := range []bool{false, true} {
			t.Run(name+strconv.FormatBool(existing), func(t *testing.T) {
				s := store.NewStore()
				router := NewRouter()
				RegisterBitmapCommands(router)
				if err := s.Set("source", &store.StringValue{Data: []byte("bytes")}, store.SetOptions{}); err != nil {
					t.Fatal(err)
				}
				if existing {
					if err := s.Set("target", &store.StringValue{Data: []byte("kept")}, store.SetOptions{}); err != nil {
						t.Fatal(err)
					}
				}
				s.ConfigureMemory(1, store.EvictionNoEviction, 70, 90, 5)
				var raw string
				switch name {
				case "SETBIT":
					raw = dumpRaceRun(t, s, router, name, "target", "0", "1")
				case "BITFIELD":
					raw = dumpRaceRun(t, s, router, name, "target", "SET", "u8", "0", "1")
				case "BITOP":
					raw = dumpRaceRun(t, s, router, name, "OR", "target", "source")
				}
				if len(raw) == 0 || raw[0] != '-' {
					t.Fatalf("acknowledged rejected allocation: %q", raw)
				}
				got, ok := s.Get("target")
				if existing {
					if !ok || got.Value.String() != "kept" {
						t.Fatal("rejected conversion replaced target")
					}
				} else if ok {
					t.Fatal("rejected creation left target")
				}
			})
		}
	}
}

func TestBitmapConversionPreservesTTLTagsAndDetachedString(t *testing.T) {
	s := store.NewStore()
	original := &store.StringValue{Data: []byte("a")}
	deadline := time.Now().Add(time.Minute).UnixMilli()
	if err := s.Set("k", original, store.SetOptions{ExpireAtMS: deadline, Tags: []string{"tag"}}); err != nil {
		t.Fatal(err)
	}
	router := NewRouter()
	RegisterBitmapCommands(router)
	dumpRaceRun(t, s, router, "SETBIT", "k", "0", "1")
	got, ok := s.Get("k")
	if !ok || got.ExpiresAt.Load() != deadline || len(got.Tags) != 1 || got.Tags[0] != "tag" {
		t.Fatal("conversion lost metadata")
	}
	if string(original.Data) != "a" {
		t.Fatal("conversion mutated detached immutable string")
	}
}
