package server

import (
	"bufio"
	"bytes"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// TestConnectionIsTrackedForTheConnectionCommands proves that a live
// connection is reported by CONNECTION.INFO, that its command counter
// advances as it issues commands, and that it stops being reported once it
// closes.
//
// The tracking is wired into the connection's lifecycle: NewConnection
// registers it, Handle counts each command against it and Close removes it.
func TestConnectionIsTrackedForTheConnectionCommands(t *testing.T) {
	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterMVCCCommands(router)
	command.RegisterServerCommands(router)

	server, client := net.Pipe()
	defer client.Close()

	c := NewConnection(9001, server, s, router, nil)

	run := func(cmd string, args ...string) string {
		argBytes := make([][]byte, 0, len(args))
		for _, a := range args {
			argBytes = append(argBytes, []byte(a))
		}
		buf := &bytes.Buffer{}
		ctx := command.NewContext(cmd, argBytes, s, resp.NewWriter(buf))
		if err := router.Execute(ctx); err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
		return buf.String()
	}

	info := run("CONNECTION.INFO", "9001")
	if !strings.Contains(info, "9001") {
		t.Fatalf("CONNECTION.INFO reply %q does not report the live connection", info)
	}

	// The connection issues one command; the counter must advance. The reply
	// has to be read: net.Pipe is unbuffered, so Handle blocks writing the
	// reply and never reaches the counter update until the client reads it.
	go func() {
		_, _ = client.Write([]byte("*1\r\n$4\r\nPING\r\n"))
	}()
	go c.Handle()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("reading the PING reply: %v", err)
	}

	// The counter is bumped after the reply is written, so wait for it.
	deadline := time.Now().Add(2 * time.Second)
	for {
		info = run("CONNECTION.INFO", "9001")
		if strings.Contains(info, ":1\r\n") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("CONNECTION.INFO reply %q does not report the issued command", info)
		}
		time.Sleep(10 * time.Millisecond)
	}

	count := run("CONNECTION.COUNT")
	if !strings.Contains(count, ":1\r\n") {
		t.Fatalf("CONNECTION.COUNT reply %q does not count the live connection", count)
	}

	c.Close()

	info = run("CONNECTION.INFO", "9001")
	if strings.Contains(info, "9001") {
		t.Fatalf("CONNECTION.INFO reply %q still reports a closed connection", info)
	}
}
