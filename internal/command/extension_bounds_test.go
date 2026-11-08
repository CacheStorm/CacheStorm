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

// Creates that allocate their slots up front refuse a count that would
// exhaust memory, and still accept a reasonable one.
func TestExtensionCreatesBoundTheirSlots(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)
	RegisterResilienceCommands(router)
	RegisterAdvancedCommands2(router)
	huge := "100000000000"
	for _, cmd := range [][]string{
		{"PARTITION.CREATE", "p", huge},
		{"PARTITIONX.CREATE", "p", huge},
		{"RINGBUFFER.CREATE", "r", huge},
		{"POOLX.CREATE", "p", huge},
		{"POOLX.CREATE", "p", "-5"},
		{"SKETCH.CREATE", "s", "4194304", "4194304"},
	} {
		if got := runCmd(t, s, router, cmd[0], cmd[1:]...); !strings.HasPrefix(got, "-ERR") {
			t.Errorf("%v: %q, want an error", cmd, got)
		}
	}
	for _, cmd := range [][]string{
		{"PARTITION.CREATE", "p", "8"}, {"PARTITIONX.CREATE", "p", "8"},
		{"RINGBUFFER.CREATE", "r", "8"}, {"POOLX.CREATE", "p", "8"},
		{"SKETCH.CREATE", "s", "100", "4"},
	} {
		if got := runCmd(t, s, router, cmd[0], cmd[1:]...); got != "+OK\r\n" {
			t.Errorf("%v: %q, want OK", cmd, got)
		}
	}
	if got := runCmd(t, s, router, "PARTITIONX.ADD", "p", "k", "v"); strings.HasPrefix(got, "-") {
		t.Errorf("PARTITIONX.ADD after create: %q", got)
	}
}

// ID.NEXTN refuses a batch it could never build, and STR.REPEAT builds its
// result in one allocation bounded like a string value.
func TestIDNextNAndStrRepeatBounds(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityCommands(router)
	RegisterTemplateCommands(router)
	if got := runCmd(t, s, router, "ID.CREATE", "bounds-seq", "1", "1", "PADDING", "100000000000"); !strings.HasPrefix(got, "-ERR") {
		t.Errorf("ID.CREATE with a huge padding: %q, want an error", got)
	}
	runCmd(t, s, router, "ID.CREATE", "bounds-seq", "1", "1", "PADDING", "4")
	for _, n := range []string{"100000000000", "-3", "0"} {
		if got := runCmd(t, s, router, "ID.NEXTN", "bounds-seq", n); !strings.HasPrefix(got, "-ERR") {
			t.Errorf("ID.NEXTN %s: %q, want an error", n, got)
		}
	}
	if got := runCmd(t, s, router, "ID.NEXTN", "bounds-seq", "3"); got != "*3\r\n$4\r\n0001\r\n$4\r\n0002\r\n$4\r\n0003\r\n" {
		t.Errorf("ID.NEXTN 3: %q", got)
	}
	start := time.Now()
	if got := runCmd(t, s, router, "STR.REPEAT", "ab", "100000000000"); !strings.HasPrefix(got, "-ERR") {
		t.Errorf("STR.REPEAT huge: %q, want an error", got)
	}
	if got := runCmd(t, s, router, "STR.REPEAT", "ab", "3"); got != "$6\r\nababab\r\n" {
		t.Errorf("STR.REPEAT ab 3: %q", got)
	}
	if got := runCmd(t, s, router, "STR.REPEAT", "ab", "-1"); got != "$0\r\n\r\n" {
		t.Errorf("STR.REPEAT ab -1: %q", got)
	}
	if got := runCmd(t, s, router, "STR.REPEAT", "x", "200000"); len(got) != 200000+len("$200000\r\n\r\n") {
		t.Errorf("STR.REPEAT x 200000 returned %d bytes", len(got))
	}
	if time.Since(start) > time.Second {
		t.Fatal("STR.REPEAT built its result by repeated concatenation")
	}
}
