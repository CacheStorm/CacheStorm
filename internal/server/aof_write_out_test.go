package server

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A reply goes out only once the writes it acknowledges are in the AOF's
// file, whatever appendfsync says, as Redis writes its AOF buffer before
// sending replies: a server killed after replying (kill -9, the OOM killer,
// a crash) keeps them. Before, they waited in a buffer for the next
// once-a-second flush, and under appendfsync no for the buffer to fill.
func TestRepliesFollowTheirAOFWrite(t *testing.T) {
	for _, policy := range []string{"everysec", "no"} {
		t.Run(policy, func(t *testing.T) {
			cfg := aofTxConfig(t)
			cfg.Persistence.AOFSync = policy
			cfg.Server.Bind, cfg.Server.Port = "127.0.0.1", 0
			s, err := newOwnedTestServer(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { stopReplServer(s) })
			c := dialRepl(t, s)
			_ = c.conn.SetDeadline(time.Now().Add(5 * time.Second))
			path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
			for _, args := range [][]string{
				{"SET", "acked-1", "v"},
				{"HSET", "acked-2", "f", "v"},
				{"MULTI"}, {"SET", "acked-3", "v"}, {"EXEC"},
			} {
				if v := c.do(args...); v == nil || v.Type == '-' {
					t.Fatalf("%v: %v", args, v)
				}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"acked-1", "acked-2", "acked-3"} {
				if !bytes.Contains(data, []byte(key)) {
					t.Errorf("%s was acknowledged but is not in the AOF file:\n%q", key, data)
				}
			}
		})
	}
}
