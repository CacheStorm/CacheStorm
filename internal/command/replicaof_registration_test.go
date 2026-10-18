package command

// Regression coverage for the REPLICAOF/SLAVEOF duplicate registration.
//
// Router.Register is plain last-write-wins (r.commands[def.Name] = def) with
// no duplicate detection. RegisterServerCommands used to register both names
// pointing at cmdSLAVEOF, a stub that reads its two arguments into `_`,
// discards them, and replies +OK unconditionally. Those entries only ever
// worked because RegisterReplicationCommands runs later in
// internal/server/server.go and overwrote them with the functional
// cmdREPLICAOF, which really calls replManager.ReplicaOf.
//
// The registrations are gone; the functional handler owns both names.
//
// Note the ORDER below. Registering in production order (server, then
// replication) would pass with or without the fix, because the stub loses
// anyway. These tests register in the ADVERSE order — replication first, server
// second — which is precisely the reorder that would let the stub shadow the
// real handler. Under that order the old code silently resolved both names to
// the stub and returned +OK while replication was never reconfigured.
//
// cmdSLAVEOF itself is deliberately left in the codebase: cmd_misc_coverage_test.go
// and cmd_deep2_coverage_test.go call it directly, and it is now unreferenced by
// the router rather than deleted.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// runReplicaCmd dispatches one command against the given router.
func runReplicaCmd(t *testing.T, router *Router, s *store.Store, args ...string) string {
	t.Helper()
	raw := make([][]byte, len(args))
	for i, a := range args {
		raw[i] = []byte(a)
	}
	var out bytes.Buffer
	ctx := NewContext(args[0], raw[1:], s, resp.NewWriter(&out))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%v: unexpected execute error: %v", args, err)
	}
	return out.String()
}

// replicaRouter registers the two command families in the ADVERSE order:
// replication first, then server. If RegisterServerCommands registers
// REPLICAOF or SLAVEOF at all, it wins here — which is exactly the shadowing
// this file guards against.
func replicaRouter() *Router {
	router := NewRouter()
	RegisterReplicationCommands(router)
	RegisterServerCommands(router)
	return router
}

// withReplicationManager installs a real manager for the duration of the test.
// The functional handler writes through it, so the test asserts on the manager's
// observable state rather than on any reply text.
func withReplicationManager(t *testing.T, s *store.Store) *ReplicationManager {
	t.Helper()
	saved := replManager
	t.Cleanup(func() { replManager = saved })

	InitReplicationManager(s)
	return GetReplicationManager()
}

// Both names must actually configure replication. The stub replies +OK without
// touching the manager, so asserting the manager state is what separates them:
// a green +OK on its own would not prove anything.
func TestReplicaofAndSlaveofConfigureReplication(t *testing.T) {
	for _, name := range []string{"REPLICAOF", "SLAVEOF"} {
		t.Run(name, func(t *testing.T) {
			s := store.NewStore()
			router := replicaRouter()
			mgr := withReplicationManager(t, s)

			if got := mgr.GetMasterHost(); got != "" {
				t.Fatalf("manager already has host %q before the command ran", got)
			}

			reply := runReplicaCmd(t, router, s, name, "cache-master.internal", "6380")
			if strings.HasPrefix(reply, "-") {
				t.Fatalf("%s was rejected: %q", name, reply)
			}

			if got := mgr.GetMasterHost(); got != "cache-master.internal" {
				t.Fatalf("%s left GetMasterHost() = %q, want %q. The command replied %q but "+
					"did not configure replication — it resolved to a handler that discards "+
					"its arguments.", name, got, "cache-master.internal", strings.TrimSpace(reply))
			}
			if got := mgr.GetMasterPort(); got != 6380 {
				t.Fatalf("%s left GetMasterPort() = %d, want 6380", name, got)
			}
		})
	}
}

// NO ONE must clear the master, not merely succeed. The stub's `host == "NO"`
// special case replies +OK without doing anything, so this catches a stub that
// special-cases the easy path.
func TestReplicaofNoOneClearsMaster(t *testing.T) {
	for _, name := range []string{"REPLICAOF", "SLAVEOF"} {
		t.Run(name, func(t *testing.T) {
			s := store.NewStore()
			router := replicaRouter()
			mgr := withReplicationManager(t, s)

			// Seed a master so the clear is observable.
			mgr.ReplicaOf("cache-master.internal", 6380)

			if reply := runReplicaCmd(t, router, s, name, "NO", "ONE"); strings.HasPrefix(reply, "-") {
				t.Fatalf("%s NO ONE was rejected: %q", name, reply)
			}

			if got := mgr.GetMasterHost(); got != "" {
				t.Fatalf("%s NO ONE left GetMasterHost() = %q, want it cleared — the command "+
					"succeeded without reconfiguring the node", name, got)
			}
		})
	}
}

// A malformed port must be rejected rather than silently accepted. This is the
// secondary branch the functional handler implements and the stub does not.
func TestReplicaofRejectsMalformedPort(t *testing.T) {
	for _, name := range []string{"REPLICAOF", "SLAVEOF"} {
		t.Run(name, func(t *testing.T) {
			s := store.NewStore()
			router := replicaRouter()
			mgr := withReplicationManager(t, s)

			reply := runReplicaCmd(t, router, s, name, "cache-master.internal", "not-a-port")
			if !strings.HasPrefix(reply, "-") {
				t.Fatalf("%s with a non-numeric port replied %q, want an error — the "+
					"functional handler validates the port", name, strings.TrimSpace(reply))
			}
			if got := mgr.GetMasterHost(); got != "" {
				t.Fatalf("%s configured master %q despite rejecting the port", name, got)
			}
		})
	}
}

// Control: the router built above is functional — it is not simply rejecting
// every command. ROLE is owned by RegisterReplicationCommands and PING by
// RegisterServerCommands, so both must resolve.
//
// MODULE deliberately does not appear here: it is owned by
// RegisterModuleCommands, which this two-family router never calls. It used to
// resolve only because server_commands.go ALSO registered it — a duplicate
// registration that has since been removed. Commands owned by a family this
// router does not build are covered by registerEveryFamily in
// router_duplicate_registration_test.go instead.
func TestDuplicateRegistrationsStillResolve(t *testing.T) {
	s := store.NewStore()
	router := replicaRouter()

	for _, name := range []string{"ROLE", "PING"} {
		reply := runReplicaCmd(t, router, s, name)
		if strings.HasPrefix(reply, "-ERR unknown command") {
			t.Fatalf("%s is no longer registered: %q", name, strings.TrimSpace(reply))
		}
	}
}
