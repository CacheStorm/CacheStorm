package persistence_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

type streamStateSourceX struct{ entries map[string]interface{} }

func (s streamStateSourceX) GetAll() map[string]interface{} { return s.entries }

func streamStateReplayX(t *testing.T, empty bool, lastID, floor string) *store.StreamValue {
	t.Helper()
	s := store.NewStreamValue(0)
	if _, err := s.Add("9-0", map[string][]byte{"f": []byte("v")}); err != nil {
		t.Fatal(err)
	}
	if empty {
		s.Delete("9-0")
	}
	s.LastID = lastID
	s.MaxDeletedID = floor
	path := filepath.Join(t.TempDir(), "rewrite.aof")
	source := streamStateSourceX{map[string]interface{}{"stream": s}}
	if err := persistence.NewAOFRewriter(persistence.AOFConfig{}, source).Rewrite(path); err != nil {
		t.Fatal(err)
	}
	commands, err := persistence.NewAOFReader().Load(path)
	if err != nil {
		t.Fatal(err)
	}
	dst := store.NewStore()
	router := command.NewRouter()
	command.RegisterStreamCommands(router)
	for _, c := range commands {
		var buf bytes.Buffer
		if err := router.ExecuteSilent(command.NewContext(c.Name, c.Args, dst, resp.NewWriter(&buf))); err != nil {
			t.Fatal(err)
		}
		v, err := resp.NewReader(&buf).ReadValue()
		if err != nil || v.Type == resp.TypeError {
			t.Fatalf("replay %s: reply=%v error=%v", c.Name, v, err)
		}
	}
	entry, ok := dst.Get("stream")
	if !ok {
		return nil
	}
	return entry.Value.(*store.StreamValue)
}

func TestAOFRewritePreservesStreamState(t *testing.T) {
	control := streamStateReplayX(t, false, "9-0", "")
	if control == nil || control.LastID != "9-0" || control.Len() != 1 {
		t.Fatal("unaffected control failed")
	}
	t.Log("CONTROL: nonempty stream preserved")
	for _, tc := range []struct {
		name          string
		empty         bool
		lastID, floor string
	}{
		{"empty", true, "9-0", ""},
		{"pristine_empty", true, "", ""},
		{"zero_id", true, "0-0", ""},
		{"deleted_top", false, "12-0", ""},
		{"deleted_floor", false, "12-0", "11-0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := streamStateReplayX(t, tc.empty, tc.lastID, tc.floor)
			if got == nil {
				t.Logf("EXPECTED: stream exists LastID=%s MaxDeletedID=%s; ACTUAL: missing", tc.lastID, tc.floor)
				t.Fatal("PROBLEM CONFIRMED")
			}
			t.Logf("EXPECTED: LastID=%s MaxDeletedID=%s; ACTUAL: LastID=%s MaxDeletedID=%s", tc.lastID, tc.floor, got.LastID, got.MaxDeletedID)
			if got.LastID != tc.lastID || got.MaxDeletedID != tc.floor {
				t.Fatal("PROBLEM CONFIRMED")
			}
			t.Log("FIX VERIFIED")
		})
	}
}
