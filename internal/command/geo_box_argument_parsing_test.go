package command

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestGeoBoxArgumentParsingX(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterGeoCommands(r)
	run := func(name string, args ...string) *resp.Value {
		t.Helper()
		raw := make([][]byte, len(args))
		for i, a := range args {
			raw[i] = []byte(a)
		}
		var b bytes.Buffer
		ctx := NewContext(name, raw, s, resp.NewWriter(&b))
		if err := r.Execute(ctx); err != nil {
			t.Fatal(err)
		}
		value, err := resp.NewReader(&b).ReadValue()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	check := func(label string, want, got interface{}) {
		t.Helper()
		t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s mismatch", label)
		}
	}
	check("seed", int64(2), run("GEOADD", "geo", "0", "0", "center", "0", "1", "north").Int)
	control := run("GEOSEARCH", "geo", "FROMMEMBER", "center", "BYRADIUS", "200", "km", "COUNT", "1")
	check("control radius count", 1, len(control.Array))
	check("control radius member", "center", string(control.Array[0].Bulk))
	got := run("GEOSEARCH", "geo", "FROMMEMBER", "center", "BYBOX", "200", "200", "km")
	check("bare BYBOX reply type", resp.TypeArray, got.Type)
	got = run("GEOSEARCHSTORE", "dest", "geo", "FROMMEMBER", "center", "BYBOX", "200", "200", "km")
	check("bare stored BYBOX reply type", resp.TypeInteger, got.Type)

	for _, origin := range [][]string{{"FROMMEMBER", "center"}, {"FROMLONLAT", "0", "0"}} {
		args := append([]string{"geo"}, origin...)
		args = append(args, "BYBOX", "200", "200", "km", "COUNT", "1")
		got = run("GEOSEARCH", args...)
		check("BYBOX COUNT reply type", resp.TypeArray, got.Type)
		check("BYBOX COUNT retained", 1, len(got.Array))
		if len(got.Array) == 1 {
			check("BYBOX nearest", "center", string(got.Array[0].Bulk))
		}
		args = append([]string{"dest", "geo"}, origin...)
		args = append(args, "BYBOX", "200", "200", "km", "COUNT", "1", "STOREDIST")
		got = run("GEOSEARCHSTORE", args...)
		check("stored BYBOX COUNT retained", int64(1), got.Int)
	}
	got = run("GEOSEARCH", "geo", "FROMMEMBER", "center", "BYBOX", "200", "200")
	check("missing unit rejected", resp.TypeError, got.Type)
	got = run("GEOSEARCHSTORE", "dest", "geo", "FROMMEMBER", "center", "BYBOX", "200", "200")
	check("stored missing unit rejected", resp.TypeError, got.Type)
	got = run("GEOSEARCH", "geo", "FROMMEMBER", "center", "BYBOX", "200", "200", "km", "WITHCOORD")
	check("following WITHCOORD retained", resp.TypeArray, got.Array[0].Type)

}
