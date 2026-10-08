package persistence_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

type auditResult struct {
	Name      string
	Want, Got any
}

func auditMust(err error) {
	if err != nil {
		panic(err)
	}
}
func auditCheck(t *testing.T, results []auditResult) {
	t.Helper()
	for _, result := range results {
		if !reflect.DeepEqual(result.Want, result.Got) {
			t.Errorf("%s: got %v, want %v", result.Name, result.Got, result.Want)
		}
	}
}
func RDB(storeFailure, verify bool) []auditResult {
	dir, err := os.MkdirTemp("", "cachestorm-evidence-rdb-")
	auditMust(err)
	defer os.RemoveAll(dir)
	src := store.NewStore()
	auditMust(src.Set("k", &store.StringValue{Data: []byte(strings.Repeat("v", 128))}, store.SetOptions{}))
	path := filepath.Join(dir, "snapshot.rdb")
	auditMust(persistence.NewRDBWriter(src, persistence.RDBConfig{Version: persistence.RDBVersion11}).Save(path))
	dst := store.NewStore()
	auditMust(persistence.NewRDBReader(dst).Load(path))
	entry, ok := dst.Get("k")
	results := []auditResult{{"control: complete snapshot restores key", true, ok && string(entry.Value.(*store.StringValue).Data) == strings.Repeat("v", 128)}}
	if !storeFailure {
		data, err := os.ReadFile(path)
		auditMust(err)
		auditMust(os.WriteFile(path, data[:len(data)-9], 0600))
		results = append(results, auditResult{"EOF marker removed", true, persistence.NewRDBReader(dst).Load(path) != nil})
		if verify {
			auditMust(os.WriteFile(path, []byte("REDIS0011"), 0600))
			results = append(results, auditResult{"header only", true, persistence.NewRDBReader(dst).Load(path) != nil})
			auditMust(os.WriteFile(path, data[:len(data)-10], 0600))
			results = append(results, auditResult{"truncated value", true, persistence.NewRDBReader(dst).Load(path) != nil})
			auditMust(os.WriteFile(path, data, 0600))
			results = append(results, auditResult{"repeat valid load", true, persistence.NewRDBReader(dst).Load(path) == nil})
		}
	} else {
		limited := store.NewStore()
		limited.ConfigureMemory(1, store.EvictionNoEviction, 80, 90, 5)
		err := persistence.NewRDBReader(limited).Load(path)
		results = append(results, auditResult{"store rejection propagated", true, err != nil}, auditResult{"rejected key absent", false, limited.Exists("k")})
		if verify {
			adequate := store.NewStore()
			adequate.ConfigureMemory(4096, store.EvictionNoEviction, 80, 90, 5)
			results = append(results, auditResult{"adequate memory", true, persistence.NewRDBReader(adequate).Load(path) == nil}, auditResult{"adequate key exists", true, adequate.Exists("k")})
			empty := store.NewStore()
			auditMust(persistence.NewRDBWriter(empty, persistence.RDBConfig{Version: persistence.RDBVersion11}).Save(path))
			results = append(results, auditResult{"empty snapshot in limited store", true, persistence.NewRDBReader(limited).Load(path) == nil})
		}
	}
	return results
}

func TestAuditRDBRequiresEOFMarker(t *testing.T) { auditCheck(t, RDB(false, true)) }

func TestAuditRDBStoreErrors(t *testing.T) { auditCheck(t, RDB(true, true)) }
