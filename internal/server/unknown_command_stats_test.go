package server

import (
	"bufio"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

// An unknown command is refused without entering the command stats, so a
// client sending made-up names cannot grow that table; a known one is
// still counted.
func TestUnknownCommandsAreNotCounted(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	s := store.NewStore()
	router := command.NewRouter()
	command.RegisterServerCommands(router)
	conn := newConnection(1, serverSide, s, router, nil, connectionOptions{})
	done := make(chan struct{})
	go func() { defer close(done); conn.Handle() }()
	defer func() { clientSide.Close(); <-done }()

	clientSide.SetDeadline(time.Now().Add(2 * time.Second))
	r := bufio.NewReader(clientSide)
	for _, cmd := range []string{"NOSUCHCMDXYZ1", "NOSUCHCMDXYZ2", "PING"} {
		if _, err := clientSide.Write([]byte("*1\r\n$" + strconv.Itoa(len(cmd)) + "\r\n" + cmd + "\r\n")); err != nil {
			t.Fatal(err)
		}
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if cmd != "PING" && !strings.HasPrefix(line, "-ERR unknown command") {
			t.Fatalf("%s: %q", cmd, line)
		}
	}
	for _, name := range []string{"NOSUCHCMDXYZ1", "NOSUCHCMDXYZ2"} {
		if st := store.GlobalMetrics.GetCommandStats(name); st != nil {
			t.Errorf("unknown command %s was counted: %v", name, st)
		}
	}
	if store.GlobalMetrics.GetCommandStats("PING") == nil {
		t.Error("PING was not counted")
	}
}
