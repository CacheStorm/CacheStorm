package server

import (
	"strings"
	"testing"
)

// Redis 8.6's HOTKEYS session, with the replies of its hotkeys.c, next to
// CacheStorm's older HOTKEYS [count].
func TestHOTKEYS(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := c.str(args...); got != want {
			t.Fatalf("%v = %q, want %q", args, got, want)
		}
	}
	c.do("HOTKEYS", "RESET")
	expect("<null>", "HOTKEYS", "GET")
	expect("<null>", "HOTKEYS", "STOP")
	expect("ERR:ERR METRICS parameter is required", "HOTKEYS", "START")
	expect("ERR:ERR METRICS parameter is required", "HOTKEYS", "START", "COUNT", "5")
	expect("ERR:ERR METRICS count must be > 0 and <= 2", "HOTKEYS", "START", "METRICS", "3", "CPU")
	expect("ERR:ERR METRICS count does not match number of metric types provided", "HOTKEYS", "START", "METRICS", "2", "CPU")
	expect("ERR:ERR METRICS CPU defined more than once!", "HOTKEYS", "START", "METRICS", "2", "CPU", "CPU")
	expect("ERR:ERR METRICS no valid metrics passed. Supported: CPU|NET", "HOTKEYS", "START", "METRICS", "1", "FOO")
	expect("ERR:ERR COUNT must be between 1 and 64", "HOTKEYS", "START", "METRICS", "1", "CPU", "COUNT", "65")
	expect("ERR:ERR DURATION must be between 1 and 1000000", "HOTKEYS", "START", "METRICS", "1", "CPU", "DURATION", "0")
	expect("ERR:ERR SAMPLE ratio must be positive", "HOTKEYS", "START", "METRICS", "1", "CPU", "SAMPLE", "0")
	expect("ERR:ERR SLOTS parameter cannot be used in non-cluster mode", "HOTKEYS", "START", "METRICS", "1", "CPU", "SLOTS", "1", "0")
	expect("ERR:ERR syntax error", "HOTKEYS", "START", "METRICS", "1", "CPU", "BOGUS")

	expect("OK", "HOTKEYS", "START", "METRICS", "2", "CPU", "NET", "COUNT", "2")
	expect("ERR:ERR hotkey tracking session already in progress", "HOTKEYS", "START", "METRICS", "1", "NET")
	expect("ERR:ERR hotkey tracking session in progress, stop tracking first", "HOTKEYS", "RESET")
	for i := 0; i < 50; i++ {
		c.do("SET", "hot", strings.Repeat("x", 100))
		c.do("GET", "hot")
	}
	c.do("SET", "warm", "v")
	c.do("MULTI")
	c.do("GET", "warm")
	c.do("EXEC")
	if !strings.Contains(c.str("INFO", "hotkeys"), "hotkeys-tracking-active:1") {
		t.Fatalf("INFO hotkeys: %q", c.str("INFO", "hotkeys"))
	}
	expect("OK", "HOTKEYS", "STOP")
	got := c.str("HOTKEYS", "GET")
	for _, want := range []string{"[[tracking-active 0 sample-ratio 1 selected-slots [[0 16383]] all-commands-all-slots-us ",
		" net-bytes-all-commands-all-slots ", " collection-start-time-unix-ms ", " total-cpu-time-user-ms ",
		" total-net-bytes ", " by-cpu-time-us [", " by-net-bytes [hot "} {
		if !strings.Contains(got, want) {
			t.Fatalf("HOTKEYS GET lacks %q: %s", want, got)
		}
	}
	if !strings.Contains(got[strings.Index(got, "by-net-bytes"):], "warm") {
		t.Fatalf("a key read inside MULTI was not tracked: %s", got)
	}
	expect("OK", "HOTKEYS", "RESET")
	expect("<null>", "HOTKEYS", "GET")
	if !strings.HasPrefix(c.str("HOTKEYS", "HELP"), "[HOTKEYS <subcommand>") {
		t.Fatalf("HOTKEYS HELP: %s", c.str("HOTKEYS", "HELP"))
	}
	expect("ERR:ERR wrong number of arguments for 'hotkeys|get' command", "HOTKEYS", "GET", "x")

	// CacheStorm's HOTKEYS [count] still lists the most accessed keys.
	if got := c.str("HOTKEYS", "1"); !strings.HasPrefix(got, "[[") {
		t.Fatalf("HOTKEYS 1 = %s", got)
	}
	if got := c.str("HOTKEYS"); !strings.HasPrefix(got, "[[") {
		t.Fatalf("HOTKEYS = %s", got)
	}
}
