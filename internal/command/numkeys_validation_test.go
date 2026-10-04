package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func numKeysRun(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterScriptCommands(router)
	RegisterListCommands(router)
	RegisterSortedSetCommands(router)
	RegisterStringCommands(router)
	RegisterKeyCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], numKeysBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v
}

func numKeysBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

// numkeys is a key count used directly as a slice length. These handlers
// checked only "is it an integer", so a negative value made
// `ArgCount() < 1+numKeys` false (a negative bound) and reached
// make([]string, numKeys) with a negative length.
func TestNumKeysRejectsNegativeValue(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup [][]string
		call  []string
	}{
		{"EVAL", nil, []string{"EVAL", "return 1", "-1"}},
		{"EVALSHA", nil, []string{"EVALSHA", "0000000000000000000000000000000000000000", "-1"}},
		{"LMPOP", [][]string{{"RPUSH", "mylist", "a", "b", "c"}}, []string{"LMPOP", "-1", "mylist", "LEFT"}},
		{"BLMPOP", [][]string{{"RPUSH", "mylist", "a", "b", "c"}}, []string{"BLMPOP", "0", "1", "-1", "mylist", "LEFT"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewStore()
			for _, c := range tc.setup {
				numKeysRun(t, s, c...)
			}

			if v := numKeysRun(t, s, tc.call...); v.Type != resp.TypeError {
				t.Fatalf("%s with a negative numkeys returned type %v, want an error", tc.name, v.Type)
			}
		})
	}
}

// Control: the guarded sibling ZMPOP already rejected this, which is the
// in-repo basis for the expected behaviour and must stay that way.
func TestZmpopNegativeNumKeysControl(t *testing.T) {
	s := store.NewStore()
	numKeysRun(t, s, "ZADD", "z", "1", "m1")

	if v := numKeysRun(t, s, "ZMPOP", "-1", "z"); v.Type != resp.TypeError {
		t.Fatalf("ZMPOP -1 z returned type %v, want an error (sibling already guards numKeys < 1)", v.Type)
	}
}

// Control: valid numkeys still work — including EVAL's legal zero-key form,
// which must not be swept up by a too-strict guard.
func TestValidNumKeysControls(t *testing.T) {
	s := store.NewStore()
	numKeysRun(t, s, "RPUSH", "mylist", "a", "b", "c")

	if v := numKeysRun(t, s, "LMPOP", "1", "mylist", "LEFT"); v.Type == resp.TypeError {
		t.Fatalf("LMPOP 1 mylist LEFT returned an error: %v", v.Err)
	}
	if v := numKeysRun(t, s, "EVAL", "return 1", "0"); v.Type == resp.TypeError {
		t.Fatalf("EVAL 'return 1' 0 returned an error — zero keys is legal: %v", v.Err)
	}
	if v := numKeysRun(t, s, "LMPOP", "2", "mylist", "mylist", "LEFT"); v.Type == resp.TypeError {
		t.Fatalf("LMPOP with 2 keys returned an error: %v", v.Err)
	}
}

// Control: a non-numeric numkeys is still an error, and a rejected request
// leaves the data untouched.
func TestNumKeysErrorAndDataControls(t *testing.T) {
	s := store.NewStore()
	numKeysRun(t, s, "RPUSH", "mylist", "a", "b", "c")

	if v := numKeysRun(t, s, "LMPOP", "abc", "mylist", "LEFT"); v.Type != resp.TypeError {
		t.Fatalf("LMPOP abc returned type %v, want an error", v.Type)
	}
	if v := numKeysRun(t, s, "EVAL", "return 1", "abc"); v.Type != resp.TypeError {
		t.Fatalf("EVAL with non-numeric numkeys returned type %v, want an error", v.Type)
	}

	if n := numKeysRun(t, s, "LLEN", "mylist").Int; n != 3 {
		t.Fatalf("LLEN = %d, want 3 — a rejected LMPOP must not pop anything", n)
	}
}
