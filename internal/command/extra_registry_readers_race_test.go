package command

import (
	"bytes"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// registryFamilyCase describes one registry family: the commands that create
// the entry, the command that mutates it and the commands that read it.
type registryFamilyCase struct {
	name   string
	setup  [][]string
	writer [][]string
	reader [][]string
}

// registryReply runs one command and returns the raw RESP reply.
func registryReply(router *Router, s *store.Store, cmd string, args ...string) (string, error) {
	buf := &bytes.Buffer{}
	ctx := NewContext(cmd, bytesArgs(args...), s, resp.NewWriter(buf))
	err := router.Execute(ctx)
	return buf.String(), err
}

// registryBulkValue extracts the value of a bulk string reply.
func registryBulkValue(reply string) string {
	if !strings.HasPrefix(reply, "$") {
		return ""
	}
	end := strings.Index(reply, "\r\n")
	if end < 0 || len(reply) < end+4 {
		return ""
	}
	return reply[end+2 : len(reply)-2]
}

// TestExtraRegistryReadersAreSynchronised drives each registry family's writer
// against its readers.
//
// The read paths took the map lock only to look the entry up and then
// released it, reading the entry's fields with no lock at all while the
// family's writer updated them under the write lock: a data race on every
// family.
//
// Run with -race this pins the data race. Each family is a subtest so a
// single family can be run in isolation (`-run 'TestX/<family>'`).
func TestExtraRegistryReadersAreSynchronised(t *testing.T) {
	cases := []registryFamilyCase{
		{
			name:   "antientropy",
			setup:  [][]string{{"ANTI_ENTROPY.SYNC", "k", "1"}},
			writer: [][]string{{"ANTI_ENTROPY.MERGE", "k", "{i}"}},
			reader: [][]string{{"ANTI_ENTROPY.DIFF", "k", "{i}"}, {"ANTI_ENTROPY.STATUS", "k"}},
		},
		{
			name:   "lww",
			setup:  [][]string{{"CRDT.LWW.SET", "k", "v"}},
			writer: [][]string{{"CRDT.LWW.SET", "k", "v2"}},
			reader: [][]string{{"CRDT.LWW.GET", "k"}},
		},
		{
			name:   "raft",
			setup:  [][]string{{"RAFT.STATE", "k", "v"}},
			writer: [][]string{{"RAFT.APPEND", "k", "e"}, {"RAFT.LEADER", "k", "l"}, {"RAFT.TERM", "k"}, {"RAFT.VOTE", "k", "c"}},
			reader: [][]string{{"RAFT.COMMIT", "k"}},
		},
		{
			name:   "batch",
			setup:  [][]string{{"BATCH.SUBMIT", "3"}},
			writer: [][]string{{"BATCH.CANCEL", "{last}"}},
			reader: [][]string{{"BATCH.STATUS", "{last}"}},
		},
		{
			name:   "deadline",
			setup:  [][]string{{"DEADLINE.SET", "k", "60000", "cb"}},
			writer: [][]string{{"DEADLINE.CANCEL", "k"}},
			reader: [][]string{{"DEADLINE.CHECK", "k"}},
		},
		{
			name:   "gateway",
			setup:  [][]string{{"GATEWAY.CREATE", "k"}},
			writer: [][]string{{"GATEWAY.ROUTE", "{last}", "pattern", "target"}},
			reader: [][]string{{"GATEWAY.METRICS", "{last}"}},
		},
		{
			name:   "bookmark",
			setup:  [][]string{{"BOOKMARK.SET", "k", "v"}},
			writer: [][]string{{"BOOKMARK.SET", "k", "v2"}},
			reader: [][]string{{"BOOKMARK.GET", "k"}},
		},
		{
			name:   "ghost",
			setup:  [][]string{{"GHOST.CREATE", "k"}},
			writer: [][]string{{"GHOST.WRITE", "k", "payload"}},
			reader: [][]string{{"GHOST.READ", "k"}},
		},
		{
			name:   "probe",
			setup:  [][]string{{"PROBE.CREATE", "k", "name", "target"}},
			writer: [][]string{{"PROBE.RUN", "k"}},
			reader: [][]string{{"PROBE.RESULTS", "k"}},
		},
		{
			name:   "canary",
			setup:  [][]string{{"CANARY.CREATE", "k", "name"}},
			writer: [][]string{{"CANARY.CHECK", "k"}},
			reader: [][]string{{"CANARY.STATUS", "k"}},
		},
		{
			name:   "ragetest",
			setup:  [][]string{{"RAGE.TEST", "k"}},
			writer: [][]string{{"RAGE.STOP", "k"}},
			reader: [][]string{{"RAGE.STATS", "k"}},
		},
		{
			name:   "tape",
			setup:  [][]string{{"TAPE.CREATE", "k"}},
			writer: [][]string{{"TAPE.WRITE", "k", "payload"}},
			reader: [][]string{{"TAPE.READ", "k"}},
		},
		{
			name:   "beacon",
			setup:  [][]string{{"BEACON.START", "k"}},
			writer: [][]string{{"BEACON.STOP", "k"}},
			reader: [][]string{{"BEACON.CHECK", "k"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewStore()
			router := NewRouter()
			RegisterExtraCommands(router)

			// The setup creates the entry the family works on. The last
			// setup reply is available to later commands as "{last}".
			last := ""
			for _, step := range tc.setup {
				reply, err := registryReply(router, s, step[0], step[1:]...)
				if err != nil {
					t.Fatalf("setup %s: unexpected error: %v", step[0], err)
				}
				if strings.HasPrefix(reply, "-") {
					t.Fatalf("setup %s: error reply %q", step[0], reply)
				}
				last = registryBulkValue(reply)
			}

			expand := func(args []string, iter int) []string {
				out := make([]string, len(args))
				for i, a := range args {
					switch a {
					case "{last}":
						out[i] = last
					case "{i}":
						out[i] = strconv.Itoa(iter)
					default:
						out[i] = a
					}
				}
				return out
			}

			// The writer must succeed, otherwise the family would look
			// clean for the wrong reason.
			for _, w := range tc.writer {
				reply, err := registryReply(router, s, w[0], expand(w[1:], 0)...)
				if err != nil {
					t.Fatalf("writer %s: unexpected error: %v", w[0], err)
				}
				if strings.HasPrefix(reply, "-") {
					t.Fatalf("writer %s: error reply %q", w[0], reply)
				}
			}
			for _, r := range tc.reader {
				reply, err := registryReply(router, s, r[0], expand(r[1:], 0)...)
				if err != nil {
					t.Fatalf("reader %s: unexpected error: %v", r[0], err)
				}
				if strings.HasPrefix(reply, "-") {
					t.Fatalf("reader %s: error reply %q", r[0], reply)
				}
			}

			const iterations = 2000

			var wg sync.WaitGroup
			start := make(chan struct{})

			for w := 0; w < 2; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					for i := 0; i < iterations; i++ {
						for _, cmd := range tc.writer {
							ctx, _ := bufCtx(cmd[0], bytesArgs(expand(cmd[1:], i)...), s)
							if err := router.Execute(ctx); err != nil {
								t.Errorf("%s: unexpected error: %v", cmd[0], err)
								return
							}
						}
					}
				}()
			}

			for r := 0; r < 2; r++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					for i := 0; i < iterations; i++ {
						for _, cmd := range tc.reader {
							ctx, _ := bufCtx(cmd[0], bytesArgs(expand(cmd[1:], i)...), s)
							if err := router.Execute(ctx); err != nil {
								t.Errorf("%s: unexpected error: %v", cmd[0], err)
								return
							}
						}
					}
				}()
			}

			close(start)
			wg.Wait()
		})
	}
}
