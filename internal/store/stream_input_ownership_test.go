package store

import (
	"reflect"
	"testing"
)

func TestStreamAddInputOwnershipX(t *testing.T) {
	check := func(label string, want, got interface{}) {
		t.Helper()
		t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s mismatch", label)
		}
	}
	add := func(v *StreamValue, id string, fields map[string][]byte) {
		t.Helper()
		if _, err := v.Add(id, fields); err != nil {
			t.Fatal(err)
		}
	}

	v := NewStreamValue(0)
	add(v, "1-0", map[string][]byte{"field": []byte("original")})
	clone := v.Clone().(*StreamValue)
	clone.GetEntryByID("1-0").Fields["field"][0] = 'X'
	check("control Clone deep copy", "original", string(v.GetEntryByID("1-0").Fields["field"]))
	fields := map[string][]byte{"field": []byte("saved"), "keep": []byte("keep")}
	added := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		<-added
		<-release
		fields["field"][0] = 'X'
		delete(fields, "keep")
		fields["new"] = []byte("later")
		close(done)
	}()
	add(v, "2-0", fields)
	close(added)
	close(release)
	<-done
	entry := v.GetEntryByID("2-0")
	check("input byte reuse", "saved", string(entry.Fields["field"]))
	check("input map deletion", "keep", string(entry.Fields["keep"]))
	_, exists := entry.Fields["new"]
	check("input map insertion", false, exists)

	empty := NewStreamValue(0)
	add(empty, "1-0", nil)
	check("nil fields stay nil", true, empty.GetEntryByID("1-0").Fields == nil)
	add(empty, "2-0", map[string][]byte{})
	check("empty fields stay nonnil", true, empty.GetEntryByID("2-0").Fields != nil)
	reused := map[string][]byte{"f": []byte("first")}
	bounded := NewStreamValue(2)
	add(bounded, "1-0", reused)
	reused["f"] = []byte("second")
	add(bounded, "2-0", reused)
	reused["f"][0] = 'X'
	check("reused map first entry", "first", string(bounded.GetEntryByID("1-0").Fields["f"]))
	check("reused map second entry", "second", string(bounded.GetEntryByID("2-0").Fields["f"]))
	add(bounded, "3-0", nil)
	check("maxlen preserved", int64(2), bounded.Len())
	check("trim preserves copied input", "second", string(bounded.GetEntryByID("2-0").Fields["f"]))

}
