package store

import (
	"strings"
	"testing"
)

func TestGeoFootprintIncludesMemberBytes(t *testing.T) {
	geo := NewGeoValue()
	base := geo.SizeOf()
	member := strings.Repeat("m", 4096)
	geo.Add(member, 0, 0)
	if got := geo.SizeOf() - base; got != 4096+32 {
		t.Fatalf("member footprint=%d", got)
	}
	copy := geo.Clone().(*GeoValue)
	copy.Add(member, 1, 1)
	if copy.SizeOf() != geo.SizeOf() {
		t.Fatal("clone or coordinate update changed footprint")
	}
	copy.Remove(member)
	if copy.SizeOf() != base || geo.SizeOf() != base+4096+32 {
		t.Fatal("removal footprint or clone isolation mismatch")
	}
	literal := &GeoValue{Points: map[string]GeoPoint{member: {}}}
	if literal.SizeOf() != geo.SizeOf() {
		t.Fatal("loaded map footprint mismatch")
	}
}
