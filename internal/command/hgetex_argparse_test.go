package command_test

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdHGETEX classified argument tokens with string heuristics
// (uppercase F-prefix, digit contents) instead of the option grammar, so
// field names starting with F swallowed the next token, digit-containing
// fields were dropped entirely (HGETEX key f1 returned a null bulk instead
// of the field's value), and the accepted EXAT/PXAT options were parsed but
// never applied.
func TestHGetExArgumentParsing(t *testing.T) {
	cases := []struct {
		name       string
		seed       map[string]string
		args       []string
		wantBulk   string
		wantArray  []string
		wantNull   bool
		wantError  bool
		wantExpiry bool
	}{
		{"plain fields control", map[string]string{"alpha": "v1", "beta": "v2"}, []string{"h", "alpha", "beta"}, "", []string{"v1", "v2"}, false, false, false},
		{"missing single field null", map[string]string{"other": "v"}, []string{"h", "nothere"}, "", nil, true, false, false},
		{"single digit field", map[string]string{"f1": "v1"}, []string{"h", "f1"}, "v1", nil, false, false, false},
		{"F-leading field mispaired", map[string]string{"foo": "v1", "bar": "v2"}, []string{"h", "foo", "bar"}, "", []string{"v1", "v2"}, false, false, false},
		{"FIELD-style names", map[string]string{"FIELD1": "v1", "plain": "v2"}, []string{"h", "FIELD1", "plain"}, "", []string{"v1", "v2"}, false, false, false},
		{"EX relative control", map[string]string{"foo": "v1"}, []string{"h", "foo", "EX", "100"}, "v1", nil, false, false, true},
		{"PERSIST control", map[string]string{"foo": "v1"}, []string{"h", "foo", "PERSIST"}, "v1", nil, false, false, false},
		{"digit field with EX", map[string]string{"f1": "v1"}, []string{"h", "f1", "EX", "100"}, "v1", nil, false, false, true},
		{"option first then digit field", map[string]string{"f1": "v1"}, []string{"h", "EX", "100", "f1"}, "v1", nil, false, false, true},
		{"EXAT applies", map[string]string{"foo": "v1"}, []string{"h", "foo", "EXAT", "4102444800"}, "v1", nil, false, false, true},
		{"PXAT applies", map[string]string{"foo": "v1"}, []string{"h", "foo", "PXAT", "4102444800000"}, "v1", nil, false, false, true},
		{"EXAT overflow rejected", map[string]string{"foo": "v1"}, []string{"h", "foo", "EXAT", "99999999999999"}, "", nil, false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewStore()
			router := command.NewRouter()
			command.RegisterHashCommands(router)
			run := func(name string, args ...string) *resp.Value {
				t.Helper()
				argv := make([][]byte, len(args))
				for i, arg := range args {
					argv[i] = []byte(arg)
				}
				var buf bytes.Buffer
				ctx := command.NewContext(name, argv, s, resp.NewWriter(&buf))
				if err := router.Execute(ctx); err != nil {
					t.Fatalf("%s execution: %v", name, err)
				}
				v, err := resp.NewReader(&buf).ReadValue()
				if err != nil {
					t.Fatalf("%s reply: %v", name, err)
				}
				return v
			}
			for field, value := range tc.seed {
				if got := run("HSET", "h", field, value); got.Type != resp.TypeInteger || got.Int != 1 {
					t.Fatalf("seed HSET %s replied %v %v, want 1", field, got.Type, got.Int)
				}
			}
			got := run("HGETEX", tc.args...)
			switch {
			case tc.wantError:
				if got.Type != resp.TypeError {
					t.Fatalf("HGETEX %v replied %v, want an error", tc.args, got.Type)
				}
				return
			case tc.wantNull:
				if got.Type != resp.TypeBulkString || !got.IsNull {
					t.Fatalf("HGETEX %v replied %v, want a null bulk", tc.args, got.Type)
				}
				return
			case tc.wantArray != nil:
				if got.Type != resp.TypeArray || len(got.Array) != len(tc.wantArray) {
					t.Fatalf("HGETEX %v replied %v with %d entries, want an array of %d", tc.args, got.Type, len(got.Array), len(tc.wantArray))
				}
				for i, want := range tc.wantArray {
					if string(got.Array[i].Bulk) != want {
						t.Errorf("HGETEX %v entry %d = %q, want %q", tc.args, i, got.Array[i].Bulk, want)
					}
				}
			default:
				if got.Type != resp.TypeBulkString || string(got.Bulk) != tc.wantBulk {
					t.Fatalf("HGETEX %v replied %v %q, want bulk %q", tc.args, got.Type, got.Bulk, tc.wantBulk)
				}
			}
			entry, ok := s.Get("h")
			if !ok {
				t.Fatal("hash key vanished")
			}
			hasExpiry := entry.ExpiresAt != 0
			if hasExpiry != tc.wantExpiry {
				t.Errorf("HGETEX %v expiry applied = %v, want %v", tc.args, hasExpiry, tc.wantExpiry)
			}
		})
	}
}
