package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
	"github.com/cachestorm/cachestorm/internal/version"
)

// TestINFOReportsTheBuildVersion pins the INFO command to the version the
// binary was stamped with.
//
// The command used to write a hardcoded "cachestorm_version:0.2.0", so every
// released binary reported 0.2.0 no matter which release it was: the value
// the release stamped into the binary was invisible to the server's own INFO
// reply, and a client reading it could not tell which build it was talking
// to.
func TestINFOReportsTheBuildVersion(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	ctx, buf := bufCtx("INFO", nil, s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("INFO: unexpected error: %v", err)
	}

	reply := buf.String()
	want := "cachestorm_version:" + version.Version
	if !strings.Contains(reply, want) {
		t.Fatalf("INFO reply does not report the build version %q: %s", want, reply)
	}
	if version.Version != "0.2.0" && strings.Contains(reply, "cachestorm_version:0.2.0") {
		t.Fatalf("INFO reply still reports the hardcoded version: %s", reply)
	}
}
