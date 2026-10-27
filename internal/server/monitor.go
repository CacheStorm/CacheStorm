package server

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
)

// monitors are the connections in MONITOR mode.
type monitors struct {
	mu    sync.Mutex
	conns map[int64]*Connection
	count atomic.Int64
}

func (m *monitors) add(c *Connection) {
	m.mu.Lock()
	if m.conns == nil {
		m.conns = make(map[int64]*Connection)
	}
	m.conns[c.ID] = c
	m.count.Store(int64(len(m.conns)))
	m.mu.Unlock()
}

func (m *monitors) remove(id int64) {
	m.mu.Lock()
	delete(m.conns, id)
	m.count.Store(int64(len(m.conns)))
	m.mu.Unlock()
}

// appendRepr appends s quoted the way Redis' sdscatrepr does.
func appendRepr(b []byte, s []byte) []byte {
	b = append(b, '"')
	for _, ch := range s {
		switch ch {
		case '\\', '"':
			b = append(b, '\\', ch)
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		case '\a':
			b = append(b, '\\', 'a')
		case '\b':
			b = append(b, '\\', 'b')
		default:
			if ch < 0x20 || ch > 0x7e {
				const hex = "0123456789abcdef"
				b = append(b, '\\', 'x', hex[ch>>4], hex[ch&0xf])
			} else {
				b = append(b, ch)
			}
		}
	}
	return append(b, '"')
}

// feedMonitors sends one MONITOR line for a command c is about to run.
func (s *Server) feedMonitors(c *Connection, db int, raw string, cmd string, args [][]byte) {
	if s == nil || s.monitor.count.Load() == 0 || !command.MonitorVisible(cmd) {
		return
	}
	now := time.Now()
	line := make([]byte, 0, 64)
	line = append(line, '+')
	line = strconv.AppendInt(line, now.Unix(), 10)
	line = append(line, '.')
	usec := strconv.FormatInt(int64(now.Nanosecond()/1000), 10)
	line = append(line, strings.Repeat("0", 6-len(usec))...)
	line = append(line, usec...)
	line = append(line, " ["...)
	line = strconv.AppendInt(line, int64(db), 10)
	line = append(line, ' ')
	line = append(line, c.conn.RemoteAddr().String()...)
	line = append(line, ']', ' ')
	line = appendRepr(line, []byte(raw))
	for _, a := range args {
		line = append(line, ' ')
		line = appendRepr(line, a)
	}
	line = append(line, '\r', '\n')
	s.monitor.mu.Lock()
	defer s.monitor.mu.Unlock()
	for id, m := range s.monitor.conns {
		if id == c.ID || m.subscriber == nil {
			continue
		}
		m.subscriber.Send(line)
	}
}
