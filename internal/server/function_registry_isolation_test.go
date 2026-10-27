package server

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func TestFunctionLibrariesAreServerLocal(t *testing.T) {
	first := startReplServer(t, nil)
	f := dialRepl(t, first)
	code := "#!lua name=isolated\nredis.register_function('local_function', function() return 'first' end)"
	if got := f.do("FUNCTION", "LOAD", code); string(got.Bulk) != "isolated" {
		t.Fatal(got)
	}
	second := startReplServer(t, nil)
	s := dialRepl(t, second)
	if got := s.do("FCALL", "local_function", "0"); got.Type != resp.TypeError {
		t.Fatalf("second server executed another server's function: %+v", got)
	}
	code = "#!lua name=isolated\nredis.register_function('local_function', function() return 'second' end)"
	if got := s.do("FUNCTION", "LOAD", code); string(got.Bulk) != "isolated" {
		t.Fatalf("independent library registration failed: %+v", got)
	}
	if got := f.do("FCALL", "local_function", "0"); string(got.Bulk) != "first" {
		t.Fatalf("first server's function replaced: %+v", got)
	}
	s.do("FUNCTION", "FLUSH")
	if got := f.do("FCALL", "local_function", "0"); string(got.Bulk) != "first" {
		t.Fatalf("another server's FUNCTION FLUSH removed the library: %+v", got)
	}
}
