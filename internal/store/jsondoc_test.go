package store

import (
	"strings"
	"testing"
)

// The expected texts were taken from Redis 8.2's RedisJSON.
func TestJSONParseAndFormatMatchRedisJSON(t *testing.T) {
	for _, c := range []struct{ in, out string }{
		{`[1.0, 1.5, 1e16, 1e15, 1.5e-5, 1e-6, 0.1, -0.0, 1E3, 9223372036854775808, -9223372036854775809, 1e300, 2.5e-300]`,
			`[1.0,1.5,1e16,1000000000000000.0,0.000015,1e-6,0.1,-0.0,1000.0,9223372036854775808,-9.223372036854776e18,1e300,2.5e-300]`},
		{`{"b":1,"a":2,"b":3}`, `{"b":3,"a":2}`},
		{"\"a\\u00e9\\u0001\\n\\t\\\"\\\\/<>&\\u2028x\"", "\"aé\\u0001\\n\\t\\\"\\\\/<>& x\""},
		{" { \"k\" : [ ] } ", `{"k":[]}`},
	} {
		n, err := ParseJSON(c.in)
		if err != nil {
			t.Fatalf("ParseJSON(%s): %v", c.in, err)
		}
		if got := string(AppendJSON(nil, n)); got != c.out {
			t.Errorf("format(%s) = %s, want %s", c.in, got, c.out)
		}
	}
	for _, c := range []struct{ in, err string }{
		{`x`, "expected value at line 1 column 1"},
		{`{"a":1`, "EOF while parsing an object at line 1 column 6"},
		{`[1,]`, "trailing comma at line 1 column 4"},
		{`01`, "invalid number at line 1 column 2"},
		{`nan`, "expected ident at line 1 column 2"},
		{`{`, "EOF while parsing an object at line 1 column 1"},
		{`1e400`, "number out of range at line 1 column 5"},
		{`1 2`, "trailing characters at line 1 column 3"},
		{strings.Repeat("[", JSONMaxDepth+1), "recursion limit exceeded at line 1 column 129"},
	} {
		if _, err := ParseJSON(c.in); err == nil || err.Error() != c.err {
			t.Errorf("ParseJSON(%q) error = %v, want %q", c.in, err, c.err)
		}
	}
}

func TestJSONObjectRemoveMovesLastKey(t *testing.T) {
	n, _ := ParseJSON(`{"a":1,"b":2,"c":3,"d":4}`)
	n.Remove("a")
	n.Remove("c")
	if got := string(AppendJSON(nil, n)); got != `{"d":4,"b":2}` {
		t.Fatalf("after removals = %s", got)
	}
}

func TestJSONPathEvaluationOrder(t *testing.T) {
	doc, _ := ParseJSON(`{"a":{"b":[1,{"c":2}]},"x":[{"c":3},{"c":"str"}],"arr":[1,2,3,2]}`)
	for _, c := range []struct{ path, want string }{
		{"$..*", `[{"b":[1,{"c":2}]},[{"c":3},{"c":"str"}],[1,2,3,2],[1,{"c":2}],1,{"c":2},2,{"c":3},{"c":"str"},3,"str",1,2,3,2]`},
		{"$..c", `[2,3,"str"]`},
		{"$..[0]", `[1,{"c":3},1]`},
		{"$.x[?(@.c>=3 || @.c=='str')].c", `[3,"str"]`},
		{"$.x[?(@.c =~ 's.*')]", `[{"c":"str"}]`},
		{"$.arr[?(@>1)]", `[2,3,2]`},
		{"$.arr[-2:]", `[3,2]`},
		{"$.arr[::2]", `[1,3]`},
		{"$['a']['b'][-1]", `[{"c":2}]`},
		{"$.a['b','zz']", `[[1,{"c":2}]]`},
		{"a..c", `[2]`},
	} {
		p, err := ParseJSONPath(c.path)
		if err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		arr := &JNode{Kind: JArray, Arr: []*JNode{}}
		for _, m := range p.Eval(doc) {
			arr.Arr = append(arr.Arr, m.Node)
		}
		if p.Legacy {
			arr.Arr = arr.Arr[:1]
		}
		if got := string(AppendJSON(nil, arr)); got != c.want {
			t.Errorf("%s = %s, want %s", c.path, got, c.want)
		}
	}
	if _, err := ParseJSONPath("$.a["); err == nil {
		t.Fatal("an unterminated bracket parsed")
	}
}

func TestJSONArithKeepsIntegers(t *testing.T) {
	one, _ := ParseJSON("1")
	half, _ := ParseJSON("1.5")
	maxInt, _ := ParseJSON("9223372036854775807")
	for _, c := range []struct {
		a, b *JNode
		mult bool
		want string
	}{
		{one, one, false, "2"},
		{one, half, false, "2.5"},
		{half, one, true, "1.5"},
		{maxInt, one, false, "-9223372036854775808"},
	} {
		n, err := JSONArith(c.a, c.b, c.mult)
		if err != nil || string(AppendJSON(nil, n)) != c.want {
			t.Errorf("arith = %v %v, want %s", n, err, c.want)
		}
	}
}
