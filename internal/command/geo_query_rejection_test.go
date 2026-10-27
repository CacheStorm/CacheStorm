package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestGeoSearchStoreRejectsMissingMember(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterGeoCommands(r)
	if got := execGeo(s, r, "GEOADD", "src", "0", "0", "seed"); got != ":1\r\n" {
		t.Fatalf("control=%q", got)
	}
	if err := s.Set("dest", &store.StringValue{Data: []byte("retained")}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	got := execGeo(s, r, "GEOSEARCHSTORE", "dest", "src", "FROMMEMBER", "absent", "BYRADIUS", "1", "km")
	if !strings.HasPrefix(got, "-ERR") {
		t.Fatalf("EXPECTED: rejected missing center | ACTUAL: %q", got)
	}
	entry, ok := s.Get("dest")
	if !ok || entry.Value.String() != "retained" {
		t.Fatal("invalid search deleted or replaced destination")
	}
}
