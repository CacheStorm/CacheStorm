package sentinel

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func TestSentinelMasterRESPArrayCount(t *testing.T) {
	s := New(Config{ID: "s1"})
	if err := s.Monitor("master", "127.0.0.1", 6379, 2); err != nil {
		t.Fatal(err)
	}
	control, err := resp.NewReader(strings.NewReader(s.handleCommand("PING") + "\r\n")).ReadValue()
	if err != nil || control.Type != resp.TypeSimpleString || control.Str != "PONG" {
		t.Fatal("unaffected control failed")
	}
	t.Log("CONTROL: PING reply parses")
	for _, flags := range [][]string{nil, {"master"}, {"master", "s_down", "o_down"}} {
		s.masters["master"].Flags = flags
		wire := s.handleCommand("SENTINEL MASTER master") + "\r\n"
		got, err := resp.NewReader(strings.NewReader(wire)).ReadValue()
		t.Logf("EXPECTED: complete array of 8 elements; ACTUAL: value=%v error=%v", got, err)
		if err != nil || got.Type != resp.TypeArray || len(got.Array) != 8 {
			t.Fatal("PROBLEM CONFIRMED")
		}
	}
	t.Log("FIX VERIFIED")
}
