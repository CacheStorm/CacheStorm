package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestCommandNamesAreCaseInsensitive(t *testing.T) {
	run := execDispatchRunner(execDispatchRouter(), store.NewStore())

	if got := run("set", "k", "v"); got != "+OK\r\n" {
		t.Fatalf("lower-case SET replied %q", got)
	}
	if got := run("GeT", "k"); got != "$1\r\nv\r\n" {
		t.Fatalf("mixed-case GET replied %q", got)
	}
	run("multi")
	run("incr", "n")
	if got := run("exec"); got != "*1\r\n:1\r\n" {
		t.Fatalf("lower-case transaction replied %q", got)
	}
}

func TestCanonicalNameDoesNotAllocateForUpperCase(t *testing.T) {
	name := "HGETALL"
	if allocs := testing.AllocsPerRun(100, func() { _ = CanonicalName(name) }); allocs != 0 {
		t.Fatalf("CanonicalName allocated %v times for an upper-case name", allocs)
	}
}
