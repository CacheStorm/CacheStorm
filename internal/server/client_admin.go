package server

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
)

// clientInfo is what CLIENT LIST reports about a connection. The connection
// updates it after every command, under its own lock, so other goroutines can
// read it while the connection runs.
type clientInfo struct {
	mu       sync.Mutex
	name     string
	libName  string
	libVer   string
	user     string
	db       int
	lastCmd  string
	lastSub  string
	lastUsed time.Time
	multi    int
	resp     int
}

// noteCommand records the command the connection is about to run, which
// CLIENT LIST shows as cmd, including for the client running CLIENT LIST.
func (c *Connection) noteCommand(cmd string, args [][]byte) {
	sub := ""
	if len(args) > 0 {
		switch cmd {
		case "CLIENT", "CONFIG", "OBJECT", "MEMORY", "SCRIPT", "FUNCTION", "XINFO", "XGROUP", "COMMAND", "ACL", "CLUSTER", "SLOWLOG", "LATENCY", "PUBSUB", "MODULE":
			sub = string(args[0])
		}
	}
	c.info.mu.Lock()
	c.info.lastCmd = cmd
	c.info.lastSub = sub
	c.info.lastUsed = time.Now()
	c.info.mu.Unlock()
}

// publishInfo records the connection state a command may have changed.
func (c *Connection) publishInfo() {
	user := "default"
	if c.aclUser != nil {
		user = c.aclUser.Name
	}
	multi := -1
	if c.transaction != nil && c.transaction.IsActive() {
		multi = len(c.transaction.GetQueued())
	}
	proto := 2
	if c.writer.IsRESP3() {
		proto = 3
	}
	c.info.mu.Lock()
	c.info.name = c.clientName
	c.info.user = user
	c.info.db = c.db
	c.info.multi = multi
	c.info.resp = proto
	c.info.mu.Unlock()
}

func (c *Connection) setLibInfo(name, ver string) {
	c.info.mu.Lock()
	if name != "" {
		c.info.libName = name
	}
	if ver != "" {
		c.info.libVer = ver
	}
	c.info.mu.Unlock()
}

// clientType is the CLIENT LIST/KILL type of the connection.
func (c *Connection) clientType() string {
	if c.handedOff.Load() {
		return "replica"
	}
	if c.subscribing.Load() {
		return "pubsub"
	}
	return "normal"
}

func (c *Connection) listLine(now time.Time, redirect int64) string {
	c.info.mu.Lock()
	info := clientInfo{name: c.info.name, libName: c.info.libName, libVer: c.info.libVer, user: c.info.user,
		db: c.info.db, lastCmd: strings.ToLower(c.info.lastCmd), lastUsed: c.info.lastUsed, multi: c.info.multi, resp: c.info.resp}
	if c.info.lastSub != "" {
		info.lastCmd += "|" + strings.ToLower(c.info.lastSub)
	}
	c.info.mu.Unlock()
	if info.lastUsed.IsZero() {
		info.lastUsed = c.createdAt
	}
	if info.user == "" {
		info.user = "default"
	}
	if info.lastCmd == "" {
		info.lastCmd = "NULL"
	}
	if info.resp == 0 {
		info.resp = 2
	}
	flags := ""
	switch c.clientType() {
	case "replica":
		flags += "S"
	case "pubsub":
		flags += "P"
	}
	if info.multi >= 0 {
		flags += "x"
	}
	if c.blocked.Load() {
		flags += "b"
	}
	if redirect >= 0 {
		flags += "t"
	}
	if flags == "" {
		flags = "N"
	}
	sub, psub, ssub := c.subCounts.Load(), c.psubCounts.Load(), c.ssubCounts.Load()
	var b strings.Builder
	b.WriteString("id=" + strconv.FormatInt(c.ID, 10))
	b.WriteString(" addr=" + c.conn.RemoteAddr().String())
	b.WriteString(" laddr=" + c.conn.LocalAddr().String())
	b.WriteString(" fd=-1 name=" + info.name)
	b.WriteString(" age=" + strconv.FormatInt(int64(now.Sub(c.createdAt)/time.Second), 10))
	b.WriteString(" idle=" + strconv.FormatInt(int64(now.Sub(info.lastUsed)/time.Second), 10))
	b.WriteString(" flags=" + flags)
	b.WriteString(" db=" + strconv.Itoa(info.db))
	b.WriteString(" sub=" + strconv.FormatInt(sub, 10))
	b.WriteString(" psub=" + strconv.FormatInt(psub, 10))
	b.WriteString(" ssub=" + strconv.FormatInt(ssub, 10))
	b.WriteString(" multi=" + strconv.Itoa(info.multi))
	b.WriteString(" watch=0 qbuf=0 qbuf-free=0 argv-mem=0 multi-mem=0 rbs=0 rbp=0 obl=0 oll=0 omem=0 tot-mem=0 events=r")
	b.WriteString(" cmd=" + info.lastCmd)
	b.WriteString(" user=" + info.user)
	if redirect < 0 {
		redirect = -1
	}
	b.WriteString(" redir=" + strconv.FormatInt(redirect, 10))
	b.WriteString(" resp=" + strconv.Itoa(info.resp))
	b.WriteString(" lib-name=" + info.libName)
	b.WriteString(" lib-ver=" + info.libVer)
	b.WriteString("\n")
	return b.String()
}

func (s *Server) clientHooks() *command.ClientHooks {
	return &command.ClientHooks{
		List:    s.clientList,
		Kill:    s.clientKill,
		Pause:   s.pauseClients,
		Unpause: s.unpauseClients,
		Unblock: s.unblockClient,
	}
}

// matchingClients returns the connections the filter selects, by id.
func (s *Server) matchingClients(caller int64, f command.ClientFilter) []*Connection {
	ids := make(map[int64]bool, len(f.IDs))
	for _, id := range f.IDs {
		ids[id] = true
	}
	now := time.Now()
	var out []*Connection
	s.conns.Range(func(_, v any) bool {
		c, ok := v.(*Connection)
		if !ok {
			return true
		}
		if len(ids) > 0 && !ids[c.ID] {
			return true
		}
		if f.SkipMe && c.ID == caller {
			return true
		}
		if f.Type != "" && c.clientType() != f.Type {
			return true
		}
		if f.Addr != "" && c.conn.RemoteAddr().String() != f.Addr {
			return true
		}
		if f.LAddr != "" && c.conn.LocalAddr().String() != f.LAddr {
			return true
		}
		if f.User != "" {
			c.info.mu.Lock()
			user := c.info.user
			c.info.mu.Unlock()
			if user == "" {
				user = "default"
			}
			if user != f.User {
				return true
			}
		}
		if f.MaxAge > 0 && int64(now.Sub(c.createdAt)/time.Second) < f.MaxAge {
			return true
		}
		out = append(out, c)
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Server) clientList(caller int64, f command.ClientFilter) string {
	now := time.Now()
	var b strings.Builder
	for _, c := range s.matchingClients(caller, f) {
		redirect := int64(-1)
		if s.tracker != nil {
			redirect = s.tracker.redirectOf(c.ID)
		}
		b.WriteString(c.listLine(now, redirect))
	}
	return b.String()
}

// clientKill closes the selected connections. The socket is closed directly;
// the connection's own goroutine notices and cleans up.
func (s *Server) clientKill(caller int64, f command.ClientFilter) int {
	n := 0
	for _, c := range s.matchingClients(caller, f) {
		if c.ID == caller {
			c.killed.Store(true)
		} else {
			_ = c.conn.Close()
		}
		n++
	}
	return n
}

// pauseState is CLIENT PAUSE: until is when the pause ends and changed is
// closed whenever the pause is lifted or replaced, waking waiting clients.
type pauseState struct {
	// untilNano mirrors until for the lock-free check every command makes.
	untilNano  atomic.Int64
	mu         sync.Mutex
	until      time.Time
	writesOnly bool
	changed    chan struct{}
}

func (s *Server) pauseClients(until time.Time, writesOnly bool) {
	s.pause.mu.Lock()
	defer s.pause.mu.Unlock()
	if !time.Now().Before(s.pause.until) {
		s.pause.until, s.pause.writesOnly = until, writesOnly
	} else {
		// A running pause is only extended, and WRITE becomes ALL, as in
		// Redis.
		if until.After(s.pause.until) {
			s.pause.until = until
		}
		if !writesOnly {
			s.pause.writesOnly = false
		}
	}
	s.pause.untilNano.Store(s.pause.until.UnixNano())
	if s.pause.changed != nil {
		close(s.pause.changed)
	}
	s.pause.changed = make(chan struct{})
}

func (s *Server) unpauseClients() {
	s.pause.mu.Lock()
	defer s.pause.mu.Unlock()
	s.pause.until = time.Time{}
	s.pause.untilNano.Store(0)
	if s.pause.changed != nil {
		close(s.pause.changed)
		s.pause.changed = nil
	}
}

// pausedCommand reports whether CLIENT PAUSE holds cmd: every client command
// under ALL, and commands that may write under WRITE. CLIENT itself always
// runs, so a paused server can be unpaused.
func pausedCommand(cmd string, writesOnly bool) bool {
	switch cmd {
	case "CLIENT", "QUIT", "RESET", "PING", "AUTH", "HELLO":
		return false
	}
	if !writesOnly {
		return true
	}
	switch cmd {
	case "EVAL", "EVALSHA", "FCALL", "PUBLISH", "SPUBLISH", "PFCOUNT", "WAIT", "EXEC":
		return true
	}
	return command.IsWriteCommand(cmd)
}

// pauseActive reports cheaply whether a CLIENT PAUSE is in force.
func (s *Server) pauseActive() bool {
	until := s.pause.untilNano.Load()
	return until != 0 && time.Now().UnixNano() < until
}

// waitWhilePaused blocks a command CLIENT PAUSE holds until the pause ends.
func (s *Server) waitWhilePaused(cmd string) {
	if s == nil {
		return
	}
	for {
		s.pause.mu.Lock()
		until, writesOnly, changed := s.pause.until, s.pause.writesOnly, s.pause.changed
		s.pause.mu.Unlock()
		wait := time.Until(until)
		if wait <= 0 || !pausedCommand(cmd, writesOnly) {
			return
		}
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-changed:
		case <-s.stopCh:
			timer.Stop()
			return
		}
		timer.Stop()
	}
}

func (s *Server) unblockClient(id int64, withError bool) bool {
	v, ok := s.conns.Load(id)
	if !ok {
		return false
	}
	c, ok := v.(*Connection)
	if !ok {
		return false
	}
	return c.unblock(withError)
}
