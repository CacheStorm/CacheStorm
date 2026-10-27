package server

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestGeoAddRejectedWritesDoNotDivergeFromAOF(t *testing.T) {
	for _, mode := range []string{"direct", "exec", "lua"} {
		t.Run(mode, func(t *testing.T) {
			cfg := aofTxConfig(t)
			s, run := aofTxStart(t, cfg)
			if got := run("GEOADD", "geo", "0", "0", "seed"); got != ":1\r\n" {
				t.Fatalf("seed=%q", got)
			}
			run("PEXPIREAT", "geo", "4102444800000")
			run("ADDTAG", "geo", "tag")
			version := s.store.WatchKey("geo")
			before := s.store.MemUsage()
			var reply string
			switch mode {
			case "exec":
				run("MULTI")
				run("GEOADD", "geo", "1", "1", "partial", "200", "0", "bad")
				reply = run("EXEC")
			case "lua":
				reply = run("EVAL", "return redis.pcall('GEOADD',KEYS[1],1,1,'partial',200,0,'bad')", "1", "geo")
			default:
				reply = run("GEOADD", "geo", "1", "1", "partial", "200", "0", "bad")
			}
			if !strings.Contains(reply, "-ERR") {
				t.Fatalf("invalid write=%q", reply)
			}
			entry, _ := s.store.Get("geo")
			expected := map[string]float64{"seed": store.GeoScore(0, 0)}
			if !reflect.DeepEqual(entry.Value.(*store.SortedSetValue).Members, expected) || s.store.GetVersion("geo") != version || s.store.MemUsage() != before {
				t.Fatal("failed write changed live data or bookkeeping")
			}
			s.store.UnwatchKey("geo")
			run("GEOADD", "geo", "1", "1", "valid")
			expected["valid"] = store.GeoScore(1, 1)
			stopReplServer(s)
			data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
			if err != nil || strings.Contains(string(data), "partial") {
				t.Fatalf("failed write entered AOF: %v", err)
			}
			restored, err := newOwnedTestServer(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			entry, ok := restored.store.Get("geo")
			if !ok || !reflect.DeepEqual(entry.Value.(*store.SortedSetValue).Members, expected) || entry.ExpiresAt.Load() != 4102444800000 || !reflect.DeepEqual(entry.TagList(), []string{"tag"}) {
				t.Fatal("live/AOF value or metadata diverged")
			}
		})
	}
}

func TestGeoAddTrustedHistoryRemainsReadable(t *testing.T) {
	for _, versioned := range []bool{false, true} {
		t.Run(strconv.FormatBool(versioned), func(t *testing.T) {
			cfg := aofTxConfig(t)
			var data []byte
			if versioned {
				data = encodeArgs("CACHESTORM.AOF", "2")
			}
			for _, args := range [][]string{
				{"GEOADD", "geo", "0", "0", "seed"},
				{"GEOADD", "geo", "NX", "XX", "1", "1", "seed"},
				{"GEOADD", "absent", "XX", "bad", "0", "ignored"},
				{"GEOADD", "nan", "NaN", "0", "legacy"},
				{"SET", "after", "loaded"},
			} {
				data = append(data, encodeArgs(args...)...)
			}
			if err := os.WriteFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"), data, 0600); err != nil {
				t.Fatal(err)
			}
			s, err := newOwnedTestServer(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			geo, ok := s.store.Get("geo")
			if !ok || !reflect.DeepEqual(geo.Value.(*store.SortedSetValue).Members, map[string]float64{"seed": store.GeoScore(0, 0)}) || s.store.Exists("absent") {
				t.Fatal("legacy conditional write changed recovered data")
			}
			nan, ok := s.store.Get("nan")
			if !ok || !math.IsNaN(nan.Value.(*store.GeoValue).Points["legacy"].Lon) || !s.store.Exists("after") {
				t.Fatal("historical data could not be recovered")
			}
		})
	}
}
