package command

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestGeoAddValidationPrecedesMutation(t *testing.T) {
	for _, args := range [][]string{
		{"1", "1", "first", "200", "0", "invalid"},
		{"1", "1", "first", "bad", "0", "invalid"},
		{"NaN", "0", "invalid"},
		{"0", "NaN", "invalid"},
		{"inf", "0", "invalid"},
		{"0", "-inf", "invalid"},
		{"NX", "XX", "0", "0", "invalid"},
		{"XX", "200", "0", "invalid"},
		{"NX", "200", "0", "seed"},
	} {
		for _, existing := range []bool{false, true} {
			t.Run(strings.Join(args, "/")+"/"+strconv.FormatBool(existing), func(t *testing.T) {
				s := store.NewStore()
				s.ConfigureMemory(0, store.EvictionNoEviction, 70, 90, 5)
				r := NewRouter()
				RegisterGeoCommands(r)
				deadline := time.Now().Add(time.Hour).UnixMilli()
				if existing {
					geo := store.NewGeoValue()
					geo.Add("seed", 0, 0)
					if err := s.Set("geo", geo, store.SetOptions{ExpireAtMS: deadline, Tags: []string{"tag"}}); err != nil {
						t.Fatal(err)
					}
				}
				version := s.WatchKey("geo")
				defer s.UnwatchKey("geo")
				before := s.MemUsage()
				records := 0
				r.SetPostExecute(func(string, [][]byte) { records++ })
				if got := execGeoAddRouter(t, s, r, append([]string{"geo"}, args...)...); !strings.HasPrefix(got, "-") {
					t.Fatalf("invalid input accepted: %q", got)
				}
				entry, ok := s.Get("geo")
				if ok != existing {
					t.Fatal("invalid input created or removed destination")
				}
				if existing {
					geo := entry.Value.(*store.GeoValue)
					if !reflect.DeepEqual(geo.Points, map[string]store.GeoPoint{"seed": {Lon: 0, Lat: 0}}) || entry.ExpiresAt.Load() != deadline || !reflect.DeepEqual(entry.Tags, []string{"tag"}) {
						t.Fatal("invalid input changed data or metadata")
					}
				}
				if records != 0 || s.GetVersion("geo") != version || s.MemUsage() != before || s.MemoryTracker().Usage() != before {
					t.Fatal("invalid input changed persistence, WATCH or memory accounting")
				}
			})
		}
	}
}

func TestGeoAddMemoryAdmissionPrecedesGrowth(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, largeMember := range []bool{false, true} {
			t.Run(strconv.FormatBool(existing)+"/"+strconv.FormatBool(largeMember), func(t *testing.T) {
				s := store.NewStore()
				s.ConfigureMemory(4096, store.EvictionNoEviction, 70, 90, 5)
				r := NewRouter()
				RegisterGeoCommands(r)
				if existing && execGeoAddRouter(t, s, r, "geo", "0", "0", "seed") != ":1\r\n" {
					t.Fatal("seed failed")
				}
				before := s.MemUsage()
				version := s.WatchKey("geo")
				defer s.UnwatchKey("geo")
				args := []string{"geo"}
				if largeMember {
					args = append(args, "1", "1", strings.Repeat("x", 8192))
				} else {
					for i := 0; i < 1000; i++ {
						args = append(args, "1", "1", strconv.Itoa(i))
					}
				}
				records := 0
				r.SetPostExecute(func(string, [][]byte) { records++ })
				if got := execGeoAddRouter(t, s, r, args...); !strings.HasPrefix(got, "-OOM") {
					t.Fatalf("oversized write accepted: %q", got)
				}
				entry, ok := s.Get("geo")
				if ok != existing || (ok && len(entry.Value.(*store.SortedSetValue).Members) != 1) {
					t.Fatal("rejected write changed members")
				}
				if records != 0 || s.GetVersion("geo") != version || s.MemUsage() != before || s.MemoryTracker().Usage() != before {
					t.Fatal("rejected write changed persistence, WATCH or memory accounting")
				}
			})
		}
	}
}

func TestGeoAddNoOpAndPublishedSnapshot(t *testing.T) {
	s := store.NewStore()
	s.ConfigureMemory(0, store.EvictionNoEviction, 70, 90, 5)
	r := NewRouter()
	RegisterGeoCommands(r)
	if execGeoAddRouter(t, s, r, "geo", "0", "0", "seed") != ":1\r\n" {
		t.Fatal("seed failed")
	}
	version := s.WatchKey("geo")
	defer s.UnwatchKey("geo")
	before, _ := s.Get("geo")
	records := 0
	r.SetPostExecute(func(string, [][]byte) { records++ })
	for _, args := range [][]string{{"geo", "0", "0", "seed"}, {"geo", "NX", "1", "1", "seed"}, {"geo", "XX", "1", "1", "absent"}, {"missing", "XX", "0", "0", "absent"}} {
		if got := execGeoAddRouter(t, s, r, args...); got != ":0\r\n" {
			t.Fatalf("no-op=%q", got)
		}
	}
	if records != 0 || s.GetVersion("geo") != version || s.Exists("missing") {
		t.Fatal("no-op changed persistence, WATCH or key existence")
	}
	if got := execGeoAddRouter(t, s, r, "geo", "CH", "1", "1", "seed", "2", "2", "seed", "3", "3", "\x00binary"); got != ":3\r\n" {
		t.Fatalf("duplicate updates=%q", got)
	}
	after, _ := s.Get("geo")
	point := before.Value.(*store.SortedSetValue).Members["seed"]
	if before.Value == after.Value || point != store.GeoScore(0, 0) || records != 1 || s.GetVersion("geo") == version {
		t.Fatal("publication changed previous snapshot or missed effect tracking")
	}
	if s.MemUsage() != s.MemoryTracker().Usage() || !bytes.Contains([]byte(after.Value.String()), []byte("binary")) {
		t.Fatal("published data or accounting mismatch")
	}
}

func execGeoAddRouter(t *testing.T, s *store.Store, r *Router, args ...string) string {
	t.Helper()
	var output bytes.Buffer
	ctx := NewContext("GEOADD", bytesArgs(args...), s, resp.NewWriter(&output))
	if err := r.Execute(ctx); err != nil {
		t.Fatal(err)
	}
	return output.String()
}
