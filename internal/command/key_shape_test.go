package command

import "testing"

func TestKeyShapeCacheSkipsUnknownCommands(t *testing.T) {
	if got := keyShapeOf("NO-SUCH-COMMAND-X"); got != keyShapeFirst {
		t.Fatalf("unknown command shape = %d, want first-key", got)
	}
	if _, ok := keyShapes.Load("NO-SUCH-COMMAND-X"); ok {
		t.Fatal("an unknown command name was cached")
	}
	for cmd, want := range map[string]keyShape{"MSET": keyShapeMSET, "PING": keyShapeNone, "GET": keyShapeFirst} {
		for range 2 {
			if got := keyShapeOf(cmd); got != want {
				t.Fatalf("%s shape = %d, want %d", cmd, got, want)
			}
		}
	}
}
