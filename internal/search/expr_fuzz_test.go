package search

import "testing"

// Any expression parses or fails without a panic, and one that parses
// evaluates on a row.
func FuzzParseExpr(f *testing.F) {
	for _, s := range []string{"lower('A',)", "(1 + foo", "1 == f() >= 2", "!(@a", "case(1,2,", "@a == b(1) 2",
		"format('%s-%s', @a, @b)", "split(@a) < 1", "substr(@a, -9, 2)", "timefmt(@b, '%c %j')", "geodistance(@a, @b)"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 200 {
			return
		}
		e, err := ParseExpr(s)
		if err != nil {
			return
		}
		if e.Lookup(func(name string) error {
			if name != "a" && name != "b" && name != "c" {
				return errNoProp
			}
			return nil
		}) != nil {
			return
		}
		r := &Row{}
		r.Set("a", "1,2")
		r.SetFloat("b", 1700000000)
		r.Set("c", "")
		r.markNull("c")
		var st EvalState
		e.Apply(r, "out", &st)
		e.Keep(r, &st)
	})
}

var errNoProp = errorString("no such property")

type errorString string

func (e errorString) Error() string { return string(e) }
