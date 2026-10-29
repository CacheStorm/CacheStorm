package server

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func (c *psClient) info(section string) map[string]string {
	c.t.Helper()
	c.send("INFO", section)
	_ = c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	v, err := resp.NewReader(c.r).ReadValue()
	if err != nil {
		c.t.Fatalf("INFO %s: %v", section, err)
	}
	text := string(v.Bulk)
	if strings.HasSuffix(text, "\r\n\r\n") {
		c.t.Fatalf("INFO %s ends with a blank line: %q", section, text)
	}
	fields := make(map[string]string)
	for _, line := range strings.Split(text, "\r\n") {
		if name, value, ok := strings.Cut(line, ":"); ok {
			fields[name] = value
		}
	}
	return fields
}

func infoInt(t *testing.T, fields map[string]string, name string) int {
	t.Helper()
	n, err := strconv.Atoi(fields[name])
	if err != nil {
		t.Fatalf("%s: %q", name, fields[name])
	}
	return n
}

// TestInfoReportsClientsAndPubSub checks the INFO fields Redis derives from
// what clients are doing: subscriptions, watched keys.
func TestInfoReportsClientsAndPubSub(t *testing.T) {
	srv := newPSServer(t)
	admin := srv.dial()
	before := admin.info("clients")

	p1, p2, s1, w := srv.dial(), srv.dial(), srv.dial(), srv.dial()
	p1.do("*3\r\n$10\r\npsubscribe\r\n$2\r\np*\r\n:1\r\n", "PSUBSCRIBE", "p*")
	p2.do("*3\r\n$10\r\npsubscribe\r\n$2\r\np*\r\n:1\r\n", "PSUBSCRIBE", "p*")
	s1.do("*3\r\n$9\r\nsubscribe\r\n$1\r\nc\r\n:1\r\n", "SUBSCRIBE", "c")
	w.do("+OK\r\n", "WATCH", "k1", "k2")

	admin.do(":1\r\n", "PUBSUB", "NUMPAT")
	clients := admin.info("clients")
	for name, want := range map[string]int{"pubsub_clients": 3, "watching_clients": 1, "total_watched_keys": 2} {
		if got := infoInt(t, clients, name) - infoInt(t, before, name); got != want {
			t.Errorf("%s grew by %d, want %d", name, got, want)
		}
	}
	stats := admin.info("stats")
	if stats["pubsub_channels"] != "1" || stats["pubsub_patterns"] != "1" || stats["pubsubshard_channels"] != "0" {
		t.Errorf("pubsub stats: channels=%s patterns=%s shard=%s", stats["pubsub_channels"], stats["pubsub_patterns"], stats["pubsubshard_channels"])
	}
	if infoInt(t, stats, "total_reads_processed") == 0 || infoInt(t, stats, "total_writes_processed") == 0 {
		t.Errorf("socket reads/writes not counted: %v", stats)
	}

	w.do("+OK\r\n", "UNWATCH")
	if got := infoInt(t, admin.info("clients"), "watching_clients") - infoInt(t, before, "watching_clients"); got != 0 {
		t.Errorf("watching_clients after UNWATCH grew by %d", got)
	}
}
