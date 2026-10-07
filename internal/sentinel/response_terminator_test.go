package sentinel

import (
	"bytes"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

type responseTerminatorConnX struct {
	net.Conn
	in  *strings.Reader
	out bytes.Buffer
}

func (c *responseTerminatorConnX) Read(p []byte) (int, error)  { return c.in.Read(p) }
func (c *responseTerminatorConnX) Write(p []byte) (int, error) { return c.out.Write(p) }
func (c *responseTerminatorConnX) Close() error                { return nil }

func TestSentinelResponsesHaveOneTerminator(t *testing.T) {
	s := New(Config{ID: "s1"})
	if err := s.Monitor("master", "127.0.0.1", 6379, 2); err != nil {
		t.Fatal(err)
	}
	control := &responseTerminatorConnX{in: strings.NewReader("PING\r\nPING\r\n")}
	s.handleConnection(control)
	r := resp.NewReader(&control.out)
	for i := 0; i < 2; i++ {
		v, err := r.ReadValue()
		if err != nil || v.Str != "PONG" {
			t.Fatal("unaffected control failed")
		}
	}
	t.Log("CONTROL: two PING replies parse independently")
	conn := &responseTerminatorConnX{in: strings.NewReader("SENTINEL GETMASTER master\r\nPING\r\n")}
	s.handleConnection(conn)
	r = resp.NewReader(&conn.out)
	first, err := r.ReadValue()
	if err != nil || first.Type != resp.TypeArray || len(first.Array) != 2 {
		t.Fatalf("GETMASTER setup: %v %v", first, err)
	}
	got, err := r.ReadValue()
	t.Logf("EXPECTED: next reply=PONG error=nil; ACTUAL: value=%v error=%v", got, err)
	if err != nil || got.Type != resp.TypeSimpleString || got.Str != "PONG" {
		t.Fatal("PROBLEM CONFIRMED")
	}
	for _, tc := range []struct {
		request string
		count   int
		kind    byte
	}{
		{"SENTINEL MASTER master", 8, byte(resp.TypeArray)},
		{"SENTINEL GETMASTER master", 2, byte(resp.TypeArray)},
		{"SENTINEL MASTER missing", 0, byte(resp.TypeError)},
		{"INFO", 0, byte(resp.TypeSimpleString)},
	} {
		c := &responseTerminatorConnX{in: strings.NewReader(tc.request + "\r\nPING\r\n")}
		s.handleConnection(c)
		r := resp.NewReader(&c.out)
		v, err := r.ReadValue()
		if err != nil || byte(v.Type) != tc.kind || len(v.Array) != tc.count {
			t.Fatalf("%s: %v %v", tc.request, v, err)
		}
		v, err = r.ReadValue()
		if err != nil || v.Str != "PONG" {
			t.Fatalf("%s next reply: %v %v", tc.request, v, err)
		}
		if _, err := r.ReadValue(); err != io.EOF {
			t.Fatalf("%s trailing bytes: %v", tc.request, err)
		}
	}
	t.Log("FIX VERIFIED")
}
