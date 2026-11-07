package command

import (
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Extension commands that size an allocation or a loop from an argument
// refuse or bound sizes a client could use to exhaust the server, and
// MATH.POW keeps the wrapping product the old loop gave.
func TestExtensionCommandsBoundTheirWork(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)
	RegisterMoreCommands(router)
	RegisterAdvancedCommands2(router)

	start := time.Now()
	for _, c := range []struct{ base, exp, want string }{
		{"2", "10", ":1024\r\n"}, {"3", "0", ":1\r\n"}, {"2", "-3", ":1\r\n"},
		{"2", "64", ":0\r\n"}, {"3", "41", ":-420491770248316829\r\n"},
		{"7", "9223372036854775807", ""},
	} {
		got := runCmd(t, s, router, "MATH.POW", c.base, c.exp)
		if c.want != "" && got != c.want {
			t.Errorf("MATH.POW %s %s: %q, want %q", c.base, c.exp, got, c.want)
		}
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("MATH.POW with a huge exponent ran a loop per unit of the exponent")
	}
	for _, cmd := range [][]string{
		{"BLOOMX.CREATE", "f", "4294967296", "3"},
		{"BLOOMX.CREATE", "f", "100", "0"},
		{"CAPTCHA.GENERATE", "100000000000"},
	} {
		if got := runCmd(t, s, router, cmd[0], cmd[1:]...); !strings.HasPrefix(got, "-ERR") {
			t.Errorf("%v: %q, want an error", cmd, got)
		}
	}
	if got := runCmd(t, s, router, "BUFFERX.CREATE", "b", "9223372036854775807"); got != "+OK\r\n" {
		t.Errorf("BUFFERX.CREATE with a huge size hint: %q", got)
	}
	if got := runCmd(t, s, router, "CAPTCHA.GENERATE", "8"); !strings.Contains(got, "answer") {
		t.Errorf("CAPTCHA.GENERATE 8: %q", got)
	}
}

// HOTKEYS lists the most accessed keys, refuses a negative count and does
// not count its own reads as accesses.
func TestHotKeys(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterStringCommands(router)
	runCmd(t, s, router, "SET", "cold", "v")
	runCmd(t, s, router, "SET", "hot", "v")
	for range 50 {
		runCmd(t, s, router, "GET", "hot")
	}
	before := runCmd(t, s, router, "HOTKEYS", "5")
	if !strings.HasPrefix(before, "*2\r\n*2\r\n$3\r\nhot\r\n:") {
		t.Fatalf("HOTKEYS 5: %q", before)
	}
	if again := runCmd(t, s, router, "HOTKEYS", "5"); again != before {
		t.Fatalf("HOTKEYS changed the counts it reports: %q then %q", before, again)
	}
	if got := runCmd(t, s, router, "HOTKEYS", "1"); strings.Count(got, "*2\r\n") != 1 {
		t.Fatalf("HOTKEYS 1: %q", got)
	}
	for _, bad := range []string{"-1", "-9223372036854775808"} {
		if got := runCmd(t, s, router, "HOTKEYS", bad); !strings.HasPrefix(got, "-ERR") {
			t.Fatalf("HOTKEYS %s: %q", bad, got)
		}
	}
}
