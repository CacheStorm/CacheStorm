package server

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
	"github.com/cachestorm/cachestorm/plugins/example-hooks"
)

// executeCommand drives srv's real router with a Context carrying the
// command name, a writer, and the full argument vector (Args[0] is the
// command echo, matching the RESP framing).
func executeCommand(srv *Server, st *store.Store, name string, args ...string) (string, error) {
	var buf bytes.Buffer
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	ctx := &command.Context{
		Store:   st,
		Command: name,
		Writer:  resp.NewWriter(&buf),
		Args:    parsed,
	}
	err := srv.router.Execute(ctx)
	return buf.String(), err
}

// End-to-end proof of the store→plugin event bridge: the composition root
// wires the store's hooks to the plugin Manager's dispatchers, so a
// registered hook consumer receives events that originate deep in the store
// — here a tag invalidation driven entirely through the command router
// (SETTAG sets the key with its tag; INVALIDATE removes the tag's keys).
func TestStoreEventsReachRegisteredHookConsumer(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Bind: "127.0.0.1", Port: 6398},
		HTTP:   config.HTTPConfig{Enabled: false},
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	consumer := examplehooks.New()
	if err := srv.pluginMgr.Register(consumer); err != nil {
		t.Fatalf("Register: %v", err)
	}

	st := srv.Store()
	if _, err := executeCommand(srv, st, "SETTAG", "bridge:tagged", "v", "t1"); err != nil {
		t.Fatalf("SETTAG: %v", err)
	}
	if _, err := executeCommand(srv, st, "INVALIDATE", "t1"); err != nil {
		t.Fatalf("INVALIDATE: %v", err)
	}

	invalidations := consumer.TagInvalidations()
	if len(invalidations) != 1 {
		t.Fatalf("consumer recorded %d tag invalidations, want 1", len(invalidations))
	}
	if invalidations[0].Tag != "t1" {
		t.Fatalf("invalidated tag = %q, want t1", invalidations[0].Tag)
	}
	if len(invalidations[0].Keys) != 1 || invalidations[0].Keys[0] != "bridge:tagged" {
		t.Fatalf("invalidated keys = %v, want [bridge:tagged]", invalidations[0].Keys)
	}
	if !strings.Contains(consumer.Log(), "tag-invalidate t1 [bridge:tagged]") {
		t.Fatalf("consumer log missing the invalidation: %q", consumer.Log())
	}
}
