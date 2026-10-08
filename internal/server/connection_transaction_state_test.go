package server_test

import (
	"bytes"
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/server"
	"github.com/cachestorm/cachestorm/internal/store"
)

type auditResult struct {
	Name      string
	Want, Got any
}

func auditCheck(t *testing.T, results []auditResult) {
	t.Helper()
	for _, result := range results {
		if !reflect.DeepEqual(result.Want, result.Got) {
			t.Errorf("%s: got %v, want %v", result.Name, result.Got, result.Want)
		}
	}
}

type memoryConn struct {
	input  *bytes.Reader
	output bytes.Buffer
}

func (c *memoryConn) Read(p []byte) (int, error)       { return c.input.Read(p) }
func (c *memoryConn) Write(p []byte) (int, error)      { return c.output.Write(p) }
func (c *memoryConn) Close() error                     { return nil }
func (c *memoryConn) LocalAddr() net.Addr              { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1} }
func (c *memoryConn) RemoteAddr() net.Addr             { return c.LocalAddr() }
func (c *memoryConn) SetDeadline(time.Time) error      { return nil }
func (c *memoryConn) SetReadDeadline(time.Time) error  { return nil }
func (c *memoryConn) SetWriteDeadline(time.Time) error { return nil }
func connectionRun(s *store.Store, commands ...[]string) string {
	var input bytes.Buffer
	for _, args := range commands {
		fmt.Fprintf(&input, "*%d\r\n", len(args))
		for _, arg := range args {
			fmt.Fprintf(&input, "$%d\r\n%s\r\n", len(arg), arg)
		}
	}
	c := &memoryConn{input: bytes.NewReader(input.Bytes())}
	r := command.NewRouter()
	command.RegisterStringCommands(r)
	command.RegisterTransactionCommands(r)
	server.NewConnection(101, c, s, r, nil).Handle()
	return c.output.String()
}
func Server(verify bool) []auditResult {
	s := store.NewStore()
	control := connectionRun(s, []string{"SET", "control", "yes"}, []string{"GET", "control"})
	actual := connectionRun(s, []string{"MULTI"}, []string{"SET", "tx", "yes"}, []string{"EXEC"})
	results := []auditResult{{"control: normal commands", "+OK\r\n$3\r\nyes\r\n", control}, {"transaction across contexts", "+OK\r\n+QUEUED\r\n*1\r\n+OK\r\n", actual}}
	if verify {
		discarded := connectionRun(s, []string{"MULTI"}, []string{"SET", "discarded", "yes"}, []string{"DISCARD"}, []string{"GET", "discarded"})
		results = append(results, auditResult{"discard", "+OK\r\n+QUEUED\r\n+OK\r\n$-1\r\n", discarded})
		empty := connectionRun(s, []string{"MULTI"}, []string{"EXEC"}, []string{"MULTI"}, []string{"EXEC"})
		results = append(results, auditResult{"empty repeated transaction", "+OK\r\n*0\r\n+OK\r\n*0\r\n", empty})
		independent := connectionRun(s, []string{"EXEC"})
		results = append(results, auditResult{"independent connection", "-ERR EXEC without MULTI\r\n", independent})
	}
	return results
}

func TestAuditConnectionTransactionState(t *testing.T) { auditCheck(t, Server(true)) }
