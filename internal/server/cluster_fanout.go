package server

import (
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// fanoutIdle is how long an unused connection to another master is kept.
const fanoutIdle = 30 * time.Second

// tagFanout sends the cluster-wide tag commands to the other masters. It
// keeps one idle connection per master, authenticated with the cluster's
// internal secret like a slot migration's, so a node that receives
// INVALIDATE does not dial every master each time.
type tagFanout struct {
	s *Server
	// timeoutMS bounds the wait for each master
	// (cluster-tag-fanout-timeout).
	timeoutMS atomic.Int64

	mu   sync.Mutex
	idle map[string]*fanoutConn

	// syncCh carries the nodes whose tag hierarchy digest differs from
	// this node's; syncAt is when each was last fetched.
	syncCh chan string
	syncAt map[string]time.Time
}

// tagSyncEvery bounds how often one node's hierarchy is fetched while the
// digests still differ (a change on its way makes them differ briefly).
const tagSyncEvery = time.Second

// digestDiffers queues node for a hierarchy fetch. The cluster calls it
// with its lock held, so it never blocks: a full queue drops the node,
// whose next PING will queue it again.
func (f *tagFanout) digestDiffers(node string) {
	select {
	case f.syncCh <- node:
	default:
	}
}

// runSync fetches and merges the hierarchy of the nodes digestDiffers
// queued, until stop closes.
func (f *tagFanout) runSync(stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case node := <-f.syncCh:
			if time.Since(f.syncAt[node]) < tagSyncEvery {
				continue
			}
			f.syncAt[node] = time.Now()
			f.s.pullTagLinks(node)
		}
	}
}

// pullTagLinks merges another node's tag hierarchy into this one's, link by
// link keeping the newer version, and hands what changed to the replicas and
// the AOF.
func (s *Server) pullTagLinks(node string) {
	r := s.fanout.Ask(node, "TAGSYNC", nil)
	if r.Err != nil || r.Reply == nil || r.Reply.Type == resp.TypeError {
		return
	}
	edges, err := command.ParseTagSync(r.Reply)
	if err != nil {
		return
	}
	s.LockWrites()
	defer s.UnlockWrites()
	ti := s.dbs.DB(0).GetTagIndex()
	for _, e := range edges {
		if !ti.ApplyEdge(e) {
			continue
		}
		cmd := command.TagEdgeCommand(e)
		args := make([][]byte, len(cmd)-1)
		for i, a := range cmd[1:] {
			args[i] = []byte(a)
		}
		if aof := s.aof(); aof != nil {
			if err := aof.AppendDB(0, cmd[0], args); err != nil {
				logger.Error().Err(err).Msg("AOF append failed")
			}
		}
		s.repl.Feed(0, cmd[0], args)
	}
}

type fanoutConn struct {
	addr   string
	conn   net.Conn
	rd     *resp.Reader
	secret string
	used   time.Time
}

func newTagFanout(s *Server, timeoutMS int) *tagFanout {
	f := &tagFanout{s: s, idle: map[string]*fanoutConn{}, syncCh: make(chan string, 64), syncAt: map[string]time.Time{}}
	f.setTimeout(timeoutMS)
	return f
}

func (f *tagFanout) setTimeout(ms int) {
	if ms <= 0 {
		ms = 1000
	}
	f.timeoutMS.Store(int64(ms))
}

// fanoutTarget is a master the command goes to.
type fanoutTarget struct {
	id   string
	addr string
}

// targets are the masters other than this node. A master the cluster has
// marked failed is left out: its replica takes its slots over, and asking
// it would only wait out the timeout.
func (f *tagFanout) targets() []fanoutTarget {
	c := f.s.router.Cluster()
	if c == nil {
		return nil
	}
	var out []fanoutTarget
	for _, sh := range c.Shards() {
		for _, n := range sh.Nodes {
			if n.Replica || n.Myself || n.Failing {
				continue
			}
			host := n.IP
			if host == "" {
				host = n.Hostname
			}
			out = append(out, fanoutTarget{id: n.ID, addr: net.JoinHostPort(host, strconv.Itoa(n.Port))})
		}
	}
	return out
}

// Send implements command.TagFanout.
func (f *tagFanout) Send(cmd string, args [][]byte) []command.FanoutReply {
	targets := f.targets()
	replies := make([]command.FanoutReply, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reply, err := f.ask(t, cmd, args)
			replies[i] = command.FanoutReply{Node: t.id, Reply: reply, Err: err}
		}()
	}
	wg.Wait()
	return replies
}

// Ask implements command.TagFanout.
func (f *tagFanout) Ask(node, cmd string, args [][]byte) command.FanoutReply {
	c := f.s.router.Cluster()
	if c == nil {
		return command.FanoutReply{Node: node, Err: errors.New("not in cluster mode")}
	}
	addr, _, ok := c.NodeAddr(node)
	if !ok {
		return command.FanoutReply{Node: node, Err: errors.New("unknown node")}
	}
	reply, err := f.ask(fanoutTarget{id: node, addr: net.JoinHostPort(addr.IP, strconv.Itoa(addr.Port))}, cmd, args)
	return command.FanoutReply{Node: node, Reply: reply, Err: err}
}

// ask sends one command to a master and reads its reply, on an idle
// connection when there is one and on a new one when that fails (the master
// may have closed it, or restarted with a new secret).
func (f *tagFanout) ask(t fanoutTarget, cmd string, args [][]byte) (*resp.Value, error) {
	secret := ""
	if c := f.s.router.Cluster(); c != nil {
		secret = c.InternalSecret()
	}
	if fc := f.take(t.addr, secret); fc != nil {
		if v, err := f.exchange(fc, cmd, args); err == nil {
			f.put(fc)
			return v, nil
		}
		_ = fc.conn.Close()
	}
	fc, err := f.dial(t.addr, secret)
	if err != nil {
		return nil, err
	}
	v, err := f.exchange(fc, cmd, args)
	if err != nil {
		_ = fc.conn.Close()
		return nil, err
	}
	f.put(fc)
	return v, nil
}

func (f *tagFanout) timeout() time.Duration {
	return time.Duration(f.timeoutMS.Load()) * time.Millisecond
}

func (f *tagFanout) dial(addr, secret string) (*fanoutConn, error) {
	d := net.Dialer{Timeout: f.timeout()}
	var conn net.Conn
	var err error
	if cfg := f.s.router.MigrateTLSConfig(); cfg != nil {
		conn, err = tls.DialWithDialer(&d, "tcp", addr, cfg)
	} else {
		conn, err = d.Dial("tcp", addr)
	}
	if err != nil {
		return nil, err
	}
	fc := &fanoutConn{addr: addr, conn: conn, rd: resp.NewReader(conn), secret: secret}
	v, err := f.exchange(fc, "AUTH", [][]byte{[]byte("internal connection"), []byte(secret)})
	if err == nil && v.Type == resp.TypeError {
		err = errors.New(v.Err)
	}
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return fc, nil
}

func (f *tagFanout) exchange(fc *fanoutConn, cmd string, args [][]byte) (*resp.Value, error) {
	_ = fc.conn.SetDeadline(time.Now().Add(f.timeout()))
	if _, err := fc.conn.Write(asmAppendCommand(nil, []byte(cmd), args)); err != nil {
		return nil, err
	}
	v, err := fc.rd.ReadValue()
	if err != nil {
		return nil, err
	}
	return v, nil
}

// take returns the idle connection to addr, if one is fresh and was
// authenticated with the current secret.
func (f *tagFanout) take(addr, secret string) *fanoutConn {
	f.mu.Lock()
	fc := f.idle[addr]
	delete(f.idle, addr)
	f.mu.Unlock()
	if fc == nil {
		return nil
	}
	if fc.secret != secret || time.Since(fc.used) > fanoutIdle {
		_ = fc.conn.Close()
		return nil
	}
	return fc
}

// put keeps fc for the next command to its master, closing the one it
// replaces.
func (f *tagFanout) put(fc *fanoutConn) {
	_ = fc.conn.SetDeadline(time.Time{})
	fc.used = time.Now()
	f.mu.Lock()
	old := f.idle[fc.addr]
	f.idle[fc.addr] = fc
	f.mu.Unlock()
	if old != nil {
		_ = old.conn.Close()
	}
}

// close drops the idle connections.
func (f *tagFanout) close() {
	f.mu.Lock()
	idle := f.idle
	f.idle = map[string]*fanoutConn{}
	f.mu.Unlock()
	for _, fc := range idle {
		_ = fc.conn.Close()
	}
}
