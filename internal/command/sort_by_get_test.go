package command

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// sortByGetRun drives SORT through a real router. It registers the string
// family too, because these tests SET the external weight/value keys.
func sortByGetRun(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterListCommands(router)
	RegisterKeyCommands(router)
	RegisterStringCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], sortLimitBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v
}

// sortByGetStrings renders a SORT reply, distinguishing a nil element (a GET
// key that does not exist) from an empty string.
func sortByGetStrings(v *resp.Value) []string {
	out := make([]string, 0, len(v.Array))
	for _, e := range v.Array {
		if e.Type == resp.TypeNull || e.IsNull {
			out = append(out, "<nil>")
		} else {
			out = append(out, string(e.Bulk))
		}
	}
	return out
}

func assertSortReply(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("returned %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("returned %v, want %v", got, want)
		}
	}
}

// BY <pattern> orders elements by the value of the external key each element
// expands to. The handler used to parse BY and discard the pattern, so the
// list came back in its original order.
func TestSortByExternalWeightIsApplied(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "b", "a", "c")
	sortByGetRun(t, s, "SET", "weight_a", "3")
	sortByGetRun(t, s, "SET", "weight_b", "1")
	sortByGetRun(t, s, "SET", "weight_c", "2")

	assertSortReply(t, sortByGetStrings(sortByGetRun(t, s, "SORT", "mylist", "BY", "weight_*")), "b", "c", "a")
}

// A missing external key weighs 0 rather than dropping the element.
func TestSortByMissingKeyWeighsZero(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "heavy", "ghost", "light")
	sortByGetRun(t, s, "SET", "weight_heavy", "9")
	sortByGetRun(t, s, "SET", "weight_light", "1")

	assertSortReply(t, sortByGetStrings(sortByGetRun(t, s, "SORT", "mylist", "BY", "weight_*")),
		"ghost", "light", "heavy")
}

// A BY pattern with no '*' behaves as if it ended in "_*".
func TestSortByPatternWithoutWildcardControl(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "a", "b")
	sortByGetRun(t, s, "SET", "w_a", "2")
	sortByGetRun(t, s, "SET", "w_b", "1")

	// The fetched SORT doc: "A pattern with no *, or a hash-field pattern
	// that resolves the same for all elements, skips sorting." The w_a/w_b
	// keys must therefore be ignored here; this used to pin the old
	// invented w_<elem> expansion, which the r17 fix removed.
	assertSortReply(t, sortByGetStrings(sortByGetRun(t, s, "SORT", "mylist", "BY", "w")), "a", "b")
}

// BY with DESC reverses the weight order.
func TestSortByDescControl(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "a", "b", "c")
	sortByGetRun(t, s, "SET", "weight_a", "1")
	sortByGetRun(t, s, "SET", "weight_b", "3")
	sortByGetRun(t, s, "SET", "weight_c", "2")

	assertSortReply(t, sortByGetStrings(sortByGetRun(t, s, "SORT", "mylist", "BY", "weight_*", "DESC")),
		"b", "c", "a")
}

// BY nosort returns the elements in their original order.
func TestSortByNosortControl(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "c", "a", "b")
	sortByGetRun(t, s, "SET", "weight_a", "1")
	sortByGetRun(t, s, "SET", "weight_b", "2")
	sortByGetRun(t, s, "SET", "weight_c", "3")

	assertSortReply(t, sortByGetStrings(sortByGetRun(t, s, "SORT", "mylist", "BY", "nosort")), "c", "a", "b")
}

// BY with ALPHA compares the external weights lexicographically.
func TestSortByAlphaControl(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "x", "y")
	sortByGetRun(t, s, "SET", "w_x", "banana")
	sortByGetRun(t, s, "SET", "w_y", "apple")

	assertSortReply(t, sortByGetStrings(sortByGetRun(t, s, "SORT", "mylist", "BY", "w_*", "ALPHA")), "y", "x")
}

// GET <pattern> projects each element through the external key and returns that
// key's value. The handler used to return the elements themselves.
func TestSortGetProjectsExternalKeys(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "a", "b")
	sortByGetRun(t, s, "SET", "data_a", "Alpha")
	sortByGetRun(t, s, "SET", "data_b", "Bravo")

	assertSortReply(t, sortByGetStrings(sortByGetRun(t, s, "SORT", "mylist", "ALPHA", "GET", "data_*")),
		"Alpha", "Bravo")
}

// "#" is the alias for the element itself, and GET may repeat: each element
// yields one value per clause, in clause order.
func TestSortGetHashTagAndRepeatedGet(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "a", "b")
	sortByGetRun(t, s, "SET", "data_a", "Alpha")
	sortByGetRun(t, s, "SET", "data_b", "Bravo")

	assertSortReply(t, sortByGetStrings(
		sortByGetRun(t, s, "SORT", "mylist", "ALPHA", "GET", "data_*", "GET", "#")),
		"Alpha", "a", "Bravo", "b")
}

// A GET key that does not exist becomes a nil element — not an empty string,
// and not a dropped slot.
func TestSortGetMissingKeyIsNil(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "a")

	assertSortReply(t, sortByGetStrings(sortByGetRun(t, s, "SORT", "mylist", "ALPHA", "GET", "absent_*")),
		"<nil>")
}

// BY and GET compose: sort by the external weight, then project.
func TestSortByAndGetCombined(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "a", "b", "c")
	sortByGetRun(t, s, "SET", "w_a", "3")
	sortByGetRun(t, s, "SET", "w_b", "1")
	sortByGetRun(t, s, "SET", "w_c", "2")
	sortByGetRun(t, s, "SET", "d_a", "Alpha")
	sortByGetRun(t, s, "SET", "d_b", "Bravo")
	sortByGetRun(t, s, "SET", "d_c", "Charlie")

	assertSortReply(t, sortByGetStrings(
		sortByGetRun(t, s, "SORT", "mylist", "BY", "w_*", "GET", "d_*")), "Bravo", "Charlie", "Alpha")
}

// GET applies after LIMIT, so paging is decided on the sorted elements.
func TestSortGetAppliesAfterLimit(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "a", "b", "c")
	sortByGetRun(t, s, "SET", "d_a", "Alpha")
	sortByGetRun(t, s, "SET", "d_b", "Bravo")
	sortByGetRun(t, s, "SET", "d_c", "Charlie")

	assertSortReply(t, sortByGetStrings(
		sortByGetRun(t, s, "SORT", "mylist", "ALPHA", "LIMIT", "0", "2", "GET", "d_*")),
		"Alpha", "Bravo")
}

// STORE with GET writes the projected values, not the elements.
func TestSortStoreWithGetControl(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "a", "b")
	sortByGetRun(t, s, "SET", "d_a", "Alpha")
	sortByGetRun(t, s, "SET", "d_b", "Bravo")

	if v := sortByGetRun(t, s, "SORT", "mylist", "ALPHA", "GET", "d_*", "STORE", "dst"); v.Type != resp.TypeInteger || v.Int != 2 {
		t.Fatalf("SORT ... GET ... STORE returned type %v int %d, want integer 2", v.Type, v.Int)
	}
	assertSortReply(t, sortByGetStrings(sortByGetRun(t, s, "LRANGE", "dst", "0", "-1")), "Alpha", "Bravo")
}

// SORT_RO rejects STORE even when GET is present.
func TestSortRoStoreRejectedWithGet(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "a")

	if v := sortByGetRun(t, s, "SORT_RO", "mylist", "GET", "#", "STORE", "dst"); v.Type != resp.TypeError {
		t.Fatalf("SORT_RO ... GET ... STORE returned type %v, want an error", v.Type)
	}
}

// ---------------------------------------------------------------------------
// Controls: unaffected paths that must keep working.
// ---------------------------------------------------------------------------

func TestSortPlainNoOptionsControl(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "3", "1", "2")
	assertSortReply(t, sortByGetStrings(sortByGetRun(t, s, "SORT", "mylist")), "1", "2", "3")
}

func TestSortAlphaDescControl(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "a", "b", "c")
	assertSortReply(t, sortByGetStrings(sortByGetRun(t, s, "SORT", "mylist", "ALPHA", "DESC")), "c", "b", "a")
}

// A beyond-length LIMIT offset clamps to the end. An earlier fix attempt in
// this function dropped that clamping and failed here.
func TestSortBeyondLengthClampControl(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "a", "b", "c")
	assertSortReply(t, sortByGetStrings(sortByGetRun(t, s, "SORT", "mylist", "ALPHA", "LIMIT", "9", "2")))
}

// A negative LIMIT offset pages from the tail.
func TestSortNegativeOffsetControl(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "a", "b", "c")
	assertSortReply(t, sortByGetStrings(sortByGetRun(t, s, "SORT", "mylist", "ALPHA", "LIMIT", "-1", "1")), "c")
}

// A non-string external key is not usable as a weight or a GET value.
func TestSortByNonStringValueControl(t *testing.T) {
	s := store.NewStore()
	sortByGetRun(t, s, "RPUSH", "mylist", "a", "b")
	sortByGetRun(t, s, "LPUSH", "w_a", "x")
	sortByGetRun(t, s, "SET", "w_b", "5")

	assertSortReply(t, sortByGetStrings(sortByGetRun(t, s, "SORT", "mylist", "BY", "w_*")), "a", "b")
}
func TestSortByGetX(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)
	RegisterStringCommands(router)
	RegisterHashCommands(router)
	RegisterKeyCommands(router)
	RegisterServerCommands(router)
	run := func(name string, args ...string) *resp.Value {
		t.Helper()
		raw := make([][]byte, len(args))
		for i, arg := range args {
			raw[i] = []byte(arg)
		}
		var output bytes.Buffer
		if err := router.Execute(NewContext(name, raw, s, resp.NewWriter(&output))); err != nil {
			t.Fatal(err)
		}
		value, err := resp.NewReader(&output).ReadValue()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	strs := func(v *resp.Value) []string {
		t.Helper()
		if v.Type != resp.TypeArray {
			t.Fatalf("expected array, got %v", v)
		}
		out := make([]string, 0, len(v.Array))
		for _, e := range v.Array {
			if e.Type != resp.TypeBulkString {
				t.Fatalf("expected bulk element, got %v", e)
			}
			out = append(out, string(e.Bulk))
		}
		return out
	}
	seedList := func() {
		if v := run("DEL", "l"); v.Type != resp.TypeInteger {
			t.Fatalf("seed del failed: %v", v)
		}
		if v := run("RPUSH", "l", "b", "a", "c"); v.Type != resp.TypeInteger || v.Int != 3 {
			t.Fatalf("seed list failed: %v", v)
		}
	}
	check := func(label string, v *resp.Value, want ...string) {
		t.Helper()
		if got := strs(v); !reflect.DeepEqual(got, want) {
			t.Errorf("FAIL %s: got %v, want %v", label, got, want)
		} else {
			t.Logf("PASS %s: %v", label, got)
		}
	}
	seedList()
	for _, kv := range [][2]string{{"weight_b", "5"}, {"weight_a", "30"}, {"weight_c", "3"}} {
		if v := run("SET", kv[0], kv[1]); v.Type != resp.TypeSimpleString {
			t.Fatalf("seed %s failed: %v", kv[0], v)
		}
	}
	check("star BY control", run("SORT", "l", "BY", "weight_*"), "c", "b", "a")
	check("star BY DESC control", run("SORT", "l", "BY", "weight_*", "DESC"), "a", "b", "c")

	seedList()
	for _, kv := range [][2]string{{"object_b", "OB"}, {"object_c", "OC"}} {
		if v := run("SET", kv[0], kv[1]); v.Type != resp.TypeSimpleString {
			t.Fatalf("seed %s failed: %v", kv[0], v)
		}
	}
	projected := strs(run("SORT", "l", "BY", "weight_*", "GET", "object_*", "GET", "#"))
	if len(projected) != 6 || projected[0] != "OC" || projected[1] != "c" || projected[2] != "OB" || projected[3] != "b" || projected[5] != "a" {
		t.Errorf("FAIL star GET control: got %v, want [OC c OB b <nil> a]", projected)
	} else {
		t.Log("PASS star GET control: [OC c OB b <nil> a]")
	}

	seedList()
	for _, kv := range [][2]string{{"static_a", "30"}, {"static_b", "5"}, {"static_c", "3"}} {
		if v := run("SET", kv[0], kv[1]); v.Type != resp.TypeSimpleString {
			t.Fatalf("seed %s failed: %v", kv[0], v)
		}
	}
	check("no-star BY skips sorting", run("SORT", "l", "BY", "static"), "b", "a", "c")

	seedList()
	for _, kv := range [][2]string{{"hw_b", "5"}, {"hw_a", "30"}, {"hw_c", "3"}} {
		if v := run("HSET", kv[0], "w", kv[1]); v.Type != resp.TypeInteger || v.Int != 1 {
			t.Fatalf("seed %s failed: %v", kv[0], v)
		}
	}
	check("hash-field BY", run("SORT", "l", "BY", "hw_*->w"), "c", "b", "a")
	check("hash-field GET", run("SORT", "l", "BY", "weight_*", "GET", "hw_*->w"), "3", "5", "30")

	check("missing BY keys stay stable", run("SORT", "l", "BY", "nomiss_*"), "b", "a", "c")
	check("plain sort control", run("SORT", "l"), "b", "a", "c")
}
