package server

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// Regression: GlobalMetrics producers (RecordCommand, RecordConnection,
// RecordDisconnection, RecordError) had no production callers, so the
// INFO/stats surfaces (METRICS.GET, STATS.CLIENTS, METRICS.CMD) reported
// zeros despite live traffic.

func TestStatsSurfacesReflectTraffic(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Bind = "127.0.0.1"
	cfg.Server.Port = 0

	s, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop(context.Background())

	port := s.listener.Addr().(*net.TCPAddr).Port
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	send := func(args ...string) (*resp.Value, error) {
		var b strings.Builder
		fmt.Fprintf(&b, "*%d\r\n", len(args))
		for _, a := range args {
			fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
		}
		if _, err := conn.Write([]byte(b.String())); err != nil {
			return nil, err
		}
		return resp.NewReader(conn).ReadValue()
	}
	// Traffic: PING + 3 SET + 2 GET.
	if v, err := send("PING"); err != nil || v == nil || v.Str != "PONG" {
		t.Fatalf("PING: got %#v, %v", v, err)
	}
	for i := 0; i < 3; i++ {
		if v, err := send("SET", fmt.Sprintf("k%d", i), fmt.Sprint(i)); err != nil || v == nil || v.Str != "OK" {
			t.Fatalf("SET k%d: got %#v, %v", i, v, err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := send("GET", fmt.Sprintf("k%d", i)); err != nil {
			t.Fatalf("GET k%d: %v", i, err)
		}
	}

	snap := flatPairs(mustSend(t, send, "METRICS.GET"))
	clients := flatPairs(mustSend(t, send, "STATS.CLIENTS"))
	cmdSet := flatPairs(mustSend(t, send, "METRICS.CMD", "SET"))

	if got := s.connCount.Load(); got != 1 {
		t.Fatalf("expected 1 active connection, got %d", got)
	}
	if intValue(clients["connected_clients"]) < 1 || intValue(clients["total_connections"]) < 1 {
		t.Fatalf("STATS.CLIENTS must count the connection, got %v", clients)
	}
	if intValue(snap["total_commands"]) < 6 {
		t.Fatalf("total_commands must count dispatched commands (>=6), got %q", snap["total_commands"])
	}
	if intValue(snap["active_connections"]) < 1 {
		t.Fatalf("active_connections must be >=1, got %q", snap["active_connections"])
	}
	if intValue(snap["total_hits"]) < 2 {
		t.Fatalf("total_hits must count the 2 GET hits, got %q", snap["total_hits"])
	}
	if intValue(cmdSet["count"]) < 3 {
		t.Fatalf("METRICS.CMD SET count must be >=3, got %q", cmdSet["count"])
	}
}

func mustSend(t *testing.T, send func(args ...string) (*resp.Value, error), args ...string) *resp.Value {
	t.Helper()
	v, err := send(args...)
	if err != nil {
		t.Fatalf("%s: %v", strings.Join(args, " "), err)
	}
	return v
}

func intValue(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}

// flatPairs flattens a RESP array of alternating key/value pairs into a
// string-keyed map. Bulk strings read from Bulk; integers render as decimals.
func flatPairs(v *resp.Value) map[string]string {
	m := map[string]string{}
	if v == nil || v.Type != resp.TypeArray {
		return m
	}
	for i := 0; i+1 < len(v.Array); i += 2 {
		k, val := v.Array[i], v.Array[i+1]
		key := k.Str
		if k.Type == resp.TypeBulkString {
			key = string(k.Bulk)
		}
		var vs string
		switch val.Type {
		case resp.TypeInteger:
			vs = strconv.FormatInt(val.Int, 10)
		case resp.TypeBulkString:
			vs = string(val.Bulk)
		}
		m[key] = vs
	}
	return m
}

// Regression: RecordRead/RecordWrite/RecordBytesIn/RecordBytesOut had no
// production callers, so total_reads, total_writes, bytes_in, and bytes_out
// reported zero despite live traffic.
func TestStatsSurfacesCountReadsWritesBytes(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Bind = "127.0.0.1"
	cfg.Server.Port = 0

	s, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop(context.Background())

	port := s.listener.Addr().(*net.TCPAddr).Port
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	send := func(args ...string) (*resp.Value, error) {
		var b strings.Builder
		fmt.Fprintf(&b, "*%d\r\n", len(args))
		for _, a := range args {
			fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
		}
		if _, err := conn.Write([]byte(b.String())); err != nil {
			return nil, err
		}
		return resp.NewReader(conn).ReadValue()
	}

	// Traffic: PING + 3 writes + 2 reads.
	if v, err := send("PING"); err != nil || v == nil || v.Str != "PONG" {
		t.Fatalf("PING: got %#v, %v", v, err)
	}
	for i := 0; i < 3; i++ {
		if v, err := send("SET", fmt.Sprintf("k%d", i), fmt.Sprint(i)); err != nil || v == nil || v.Str != "OK" {
			t.Fatalf("SET k%d: got %#v, %v", i, v, err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := send("GET", fmt.Sprintf("k%d", i)); err != nil {
			t.Fatalf("GET k%d: %v", i, err)
		}
	}

	snap := flatPairs(mustSend(t, send, "METRICS.GET"))

	if intValue(snap["total_writes"]) < 3 {
		t.Fatalf("total_writes must count the 3 SETs, got %q", snap["total_writes"])
	}
	if intValue(snap["total_reads"]) < 3 {
		t.Fatalf("total_reads must count PING + 2 GETs, got %q", snap["total_reads"])
	}
	if intValue(snap["bytes_in"]) <= 0 {
		t.Fatalf("bytes_in must count received bytes, got %q", snap["bytes_in"])
	}
	if intValue(snap["bytes_out"]) <= 0 {
		t.Fatalf("bytes_out must count sent bytes, got %q", snap["bytes_out"])
	}
}
