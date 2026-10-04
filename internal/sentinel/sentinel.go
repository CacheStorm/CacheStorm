package sentinel

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/logger"
)

type MasterState int

const (
	MasterStateNone MasterState = iota
	MasterStateOK
	MasterStateSDown
	MasterStateODown
)

type Sentinel struct {
	mu            sync.RWMutex
	id            string
	addr          string
	port          int
	masters       map[string]*MasterInfo
	sentinels     map[string][]*SentinelPeer
	running       atomic.Bool
	stopCh        chan struct{}
	wg            sync.WaitGroup
	downAfter     time.Duration
	parallelSyncs int
	failoverTime  time.Duration
	quorum        int
	seeds         []string
	onFailover    func(master string, newAddr string, newPort int)
}

type MasterInfo struct {
	Name          string
	Addr          string
	Port          int
	State         MasterState
	LastPing      time.Time
	LastOkPing    time.Time
	NumReplicas   int
	NumSentinels  int
	Flags         []string
	Replicas      []*ReplicaInfo
	FailoverState string
	Leader        string
	Epoch         int64
	Quorum        int
}

type ReplicaInfo struct {
	Addr     string
	Port     int
	State    string
	LastPing time.Time
	Offset   int64
	Lag      int64
}

type SentinelPeer struct {
	ID       string
	Addr     string
	Port     int
	LastSeen time.Time
	RunID    string
	Epoch    int64
}

type Config struct {
	ID            string
	Addr          string
	Port          int
	DownAfter     time.Duration
	ParallelSyncs int
	FailoverTime  time.Duration
	Quorum        int
	// Seeds are "addr:port" addresses of sentinels to announce ourselves to on
	// every gossip tick. A sentinel has no other way to learn that peers exist,
	// so discovery starts from these bootstrap addresses and spreads: each peer
	// we reach learns about us via SENTINEL HELLO, and its reply tells us who
	// it is.
	Seeds []string
}

func New(cfg Config) *Sentinel {
	if cfg.DownAfter == 0 {
		cfg.DownAfter = 30 * time.Second
	}
	if cfg.ParallelSyncs == 0 {
		cfg.ParallelSyncs = 1
	}
	if cfg.FailoverTime == 0 {
		cfg.FailoverTime = 3 * time.Minute
	}
	if cfg.Quorum == 0 {
		cfg.Quorum = 2
	}

	return &Sentinel{
		id:            cfg.ID,
		addr:          cfg.Addr,
		port:          cfg.Port,
		masters:       make(map[string]*MasterInfo),
		sentinels:     make(map[string][]*SentinelPeer),
		stopCh:        make(chan struct{}),
		downAfter:     cfg.DownAfter,
		parallelSyncs: cfg.ParallelSyncs,
		failoverTime:  cfg.FailoverTime,
		quorum:        cfg.Quorum,
		seeds:         cfg.Seeds,
	}
}

func (s *Sentinel) Start() error {
	s.mu.Lock()
	if s.running.Load() {
		s.mu.Unlock()
		return fmt.Errorf("sentinel already started")
	}
	// stopCh is one-shot: Stop closes it and never recreates it. Without this
	// a restart would launch loops reading an already-closed channel (they exit
	// at once) and the next Stop would close it again — "close of closed
	// channel". The server wires Start/Stop per Server instance, so a restart
	// is reachable.
	select {
	case <-s.stopCh:
		s.stopCh = make(chan struct{})
	default:
	}
	s.running.Store(true)
	s.mu.Unlock()

	s.wg.Add(1)
	go s.monitorLoop()

	s.wg.Add(1)
	go s.gossipLoop()

	logger.Info().Str("id", s.id).Msg("Sentinel started")
	return nil
}

func (s *Sentinel) Stop() {
	if !s.running.CompareAndSwap(true, false) {
		return
	}
	close(s.stopCh)
	s.wg.Wait()
	logger.Info().Msg("Sentinel stopped")
}

func (s *Sentinel) monitorLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkMasters()
		}
	}
}

func (s *Sentinel) gossipLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.gossipSentinels()
		}
	}
}

func (s *Sentinel) checkMasters() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for name, master := range s.masters {
		if s.isReachable(master.Addr, master.Port) {
			master.State = MasterStateOK
			master.LastOkPing = time.Now()
			master.Flags = []string{"master"}
		} else {
			if master.State == MasterStateOK || master.State == MasterStateNone {
				master.State = MasterStateSDown
				master.Flags = []string{"master", "s_down"}
				logger.Warn().
					Str("master", name).
					Msg("Master marked as subjectively down")
			}

			// checkMasters holds s.mu for its whole body, so read the peers
			// directly and use the lock-free core — calling checkODown here
			// deadlocked on its nested RLock.
			if s.checkODownLocked(master, s.sentinels[name]) {
				master.State = MasterStateODown
				master.Flags = []string{"master", "s_down", "o_down"}
				logger.Error().
					Str("master", name).
					Msg("Master marked as objectively down")

				go func(name string, master *MasterInfo) {
					defer logger.RecoverPanic("sentinel-failover")
					s.startFailover(name, master)
				}(name, master)
			}
		}
	}
}

func (s *Sentinel) isReachable(addr string, port int) bool {
	conn, err := net.DialTimeout("tcp",
		net.JoinHostPort(addr, strconv.Itoa(port)),
		2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (s *Sentinel) checkODown(master *MasterInfo) bool {
	s.mu.RLock()
	peers := s.sentinels[master.Name]
	s.mu.RUnlock()

	return s.checkODownLocked(master, peers)
}

// checkODownLocked is the lock-free core of checkODown. checkMasters already
// holds the exclusive lock for its whole body and sync.RWMutex is not
// reentrant, so calling checkODown from there parked forever on its nested
// RLock: a new RLock cannot be granted while a writer holds the lock, and the
// deferred Unlock that would release it sits downstream of the stuck call. That
// wedged the monitor on the first unreachable master. Callers already holding
// s.mu pass in the peer slice they read themselves.
func (s *Sentinel) checkODownLocked(master *MasterInfo, peers []*SentinelPeer) bool {
	downCount := 0
	for _, peer := range peers {
		// Skip this sentinel's own entry: the "+1" below already counts it, and
		// gossipSentinels registers self as a peer for liveness bookkeeping.
		// Counting both would let a lone sentinel meet a quorum of 2.
		if peer.ID == s.id {
			continue
		}
		if time.Since(peer.LastSeen) < s.downAfter {
			downCount++
		}
	}

	// Honour the quorum Monitor stored for THIS master. Comparing against the
	// sentinel-wide s.quorum ignored MasterInfo.Quorum entirely, so a caller
	// passing quorum to Monitor had that argument silently discarded and every
	// master was governed by one global number. Fall back to the global value
	// only when the master has no quorum of its own.
	quorum := master.Quorum
	if quorum <= 0 {
		quorum = s.quorum
	}

	return downCount+1 >= quorum
}

func (s *Sentinel) startFailover(name string, master *MasterInfo) {
	s.mu.Lock()
	if master.FailoverState != "" {
		s.mu.Unlock()
		return
	}
	master.FailoverState = "waiting"
	s.mu.Unlock()

	logger.Info().Str("master", name).Msg("Starting failover")

	// Wait for failover delay (configurable, default 5s)
	failoverDelay := s.failoverTime
	if failoverDelay == 0 {
		failoverDelay = 5 * time.Second
	}

	select {
	case <-time.After(failoverDelay):
	case <-s.stopCh:
		return
	}

	var bestReplica *ReplicaInfo
	s.mu.RLock()
	for _, r := range master.Replicas {
		if bestReplica == nil || r.Offset > bestReplica.Offset {
			bestReplica = r
		}
	}
	s.mu.RUnlock()

	if bestReplica == nil {
		logger.Error().Str("master", name).Msg("No replica available for failover")
		s.mu.Lock()
		master.FailoverState = ""
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	master.Addr = bestReplica.Addr
	master.Port = bestReplica.Port
	master.State = MasterStateOK
	master.FailoverState = ""
	master.Flags = []string{"master"}
	master.Epoch++
	s.mu.Unlock()

	logger.Info().
		Str("master", name).
		Str("new_addr", bestReplica.Addr).
		Int("new_port", bestReplica.Port).
		Msg("Failover completed")

	if s.onFailover != nil {
		s.onFailover(name, bestReplica.Addr, bestReplica.Port)
	}
}

// recordPeer registers a peer announced by SENTINEL HELLO, against every
// monitored master, and refreshes its LastSeen if it is already known. This is
// the receiving half of automatic discovery: the announcing sentinel is added
// to our own table with no manual registration.
func (s *Sentinel) recordPeer(addr string, port int) {
	id := fmt.Sprintf("%s:%d", addr, port)

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for name := range s.masters {
		peers := s.sentinels[name]
		found := false
		for _, p := range peers {
			if p.ID == id {
				p.LastSeen = now
				found = true
				break
			}
		}
		if !found {
			s.sentinels[name] = append(peers, &SentinelPeer{
				ID: id, Addr: addr, Port: port, LastSeen: now,
			})
		}
	}
}

// announceTo tells a peer who we are with SENTINEL HELLO, so it records us in
// its own table with no manual registration. The reply carries that peer's
// identity, which we record in turn — one round trip makes the two sentinels
// aware of each other.
//
// Unlike pingPeer (which only tests liveness), this changes the peer's view of
// the cluster, so it is what gossip uses to spread discovery.
func (s *Sentinel) announceTo(addr string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(addr, strconv.Itoa(port)), 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	req := fmt.Sprintf("SENTINEL HELLO %s %d\r\n", s.addr, s.port)
	if _, err := conn.Write([]byte(req)); err != nil {
		return false
	}

	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return false
	}

	// Reply shape: "+OK <id> <addr> <port>".
	fields := strings.Fields(string(buf[:n]))
	if len(fields) < 4 || fields[0] != "+OK" {
		return false
	}
	replyPort, err := strconv.Atoi(fields[3])
	if err != nil || replyPort <= 0 || replyPort > 65535 {
		return false
	}
	s.recordPeer(fields[2], replyPort)
	return true
}

// parseSeed splits a "addr:port" seed into its parts.
func parseSeed(seed string) (string, int, bool) {
	host, portStr, err := net.SplitHostPort(strings.TrimSpace(seed))
	if err != nil {
		return "", 0, false
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, false
	}
	return host, port, true
}

// pingPeer sends a PING to another sentinel and reports whether it answered.
// This is the same wire protocol Serve/handleCommand already speaks, so any
// CacheStorm sentinel (or a plain listener answering +PONG) is a valid peer.
func (s *Sentinel) pingPeer(p *SentinelPeer) bool {
	addr := p.Addr
	if addr == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(addr, strconv.Itoa(p.Port)), 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("PING\r\n")); err != nil {
		return false
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return false
	}
	return strings.Contains(string(buf[:n]), "+PONG")
}

// gossipSentinels maintains the peer table for every monitored master.
//
// It does three things on each tick: registers this sentinel as its own peer so
// the table is populated at all (it used to be empty forever, which is why
// CKQUORUM always answered 1 and checkODown's downCount was always 0); actively
// PINGs every known peer and refreshes LastSeen when it answers; and drops peers
// that have been silent past the retention window so the table cannot grow
// without bound.
//
// Self is registered as a peer for liveness bookkeeping, but the "+1" in
// checkODownLocked and CKQUORUM already counts this sentinel, so those counting
// sites skip the entry whose ID is s.id. Otherwise self would be counted twice
// and a quorum of 2 would be met by a lone sentinel.
//
// NOTE: this maintains peers that are already known. Genuine discovery of NEW
// peers needs a transport to hear them announce themselves (Redis uses the
// __sentinel__:hello pub/sub channel); CacheStorm's sentinel has no such
// transport and Sentinel.Serve is not yet wired into the server, so peers are
// currently only reachable once something has registered them.
func (s *Sentinel) gossipSentinels() {
	s.mu.Lock()
	masters := make([]string, 0, len(s.masters))
	for name := range s.masters {
		masters = append(masters, name)
	}

	now := time.Now()
	type peerRef struct {
		master string
		peer   *SentinelPeer
	}
	var toProbe []peerRef

	for _, name := range masters {
		peers := s.sentinels[name]
		hasSelf := false
		for _, p := range peers {
			if p.ID == s.id {
				hasSelf = true
				p.LastSeen = now
				continue
			}
			toProbe = append(toProbe, peerRef{master: name, peer: p})
		}
		if !hasSelf {
			peers = append(peers, &SentinelPeer{
				ID:       s.id,
				Addr:     s.addr,
				Port:     s.port,
				LastSeen: now,
			})
			s.sentinels[name] = peers
		}
	}
	s.mu.Unlock()

	// Probe outside the lock: a dial can take up to 2s and checkMasters must
	// not be blocked behind it. Results are applied under the lock below so
	// LastSeen is never written concurrently with a reader.
	var alive []*SentinelPeer
	for _, pr := range toProbe {
		if s.pingPeer(pr.peer) {
			alive = append(alive, pr.peer)
		}
	}

	// Announce ourselves to the configured seeds so they record us without any
	// manual registration, and record whoever answers so we learn them too.
	// This is the discovery path: without it a sentinel has no way to learn that
	// any peer exists. Runs outside the lock because announceTo -> recordPeer
	// takes s.mu itself.
	for _, seed := range s.seeds {
		host, port, ok := parseSeed(seed)
		if !ok {
			logger.Warn().Str("seed", seed).Msg("skipping malformed sentinel seed")
			continue
		}
		if net.JoinHostPort(host, strconv.Itoa(port)) == net.JoinHostPort(s.addr, strconv.Itoa(s.port)) {
			continue // announcing to ourselves
		}
		s.announceTo(host, port)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range alive {
		p.LastSeen = now
	}

	// Prune peers that have gone quiet past the retention window.
	retention := 3 * s.downAfter
	if retention <= 0 {
		retention = 90 * time.Second
	}
	for _, name := range masters {
		peers := s.sentinels[name]
		kept := make([]*SentinelPeer, 0, len(peers))
		for _, p := range peers {
			if now.Sub(p.LastSeen) <= retention {
				kept = append(kept, p)
			}
		}
		s.sentinels[name] = kept
	}
}

func (s *Sentinel) Monitor(name, addr string, port, quorum int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.masters[name]; exists {
		return fmt.Errorf("master '%s' already monitored", name)
	}

	s.masters[name] = &MasterInfo{
		Name:     name,
		Addr:     addr,
		Port:     port,
		State:    MasterStateNone,
		Quorum:   quorum,
		Replicas: make([]*ReplicaInfo, 0),
	}

	logger.Info().
		Str("name", name).
		Str("addr", addr).
		Int("port", port).
		Msg("Monitoring master")

	return nil
}

func (s *Sentinel) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.masters[name]; !exists {
		return fmt.Errorf("master '%s' not monitored", name)
	}

	delete(s.masters, name)
	delete(s.sentinels, name)

	return nil
}

func (s *Sentinel) Masters() []*MasterInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*MasterInfo, 0, len(s.masters))
	for _, m := range s.masters {
		result = append(result, m)
	}
	return result
}

func (s *Sentinel) GetMaster(name string) (*MasterInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.masters[name]
	return m, ok
}

func (s *Sentinel) GetMasterAddr(name string) (string, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.masters[name]
	if !ok {
		return "", 0, fmt.Errorf("master '%s' not monitored", name)
	}

	return m.Addr, m.Port, nil
}

func (s *Sentinel) CKQUORUM(name string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.masters[name]
	if !ok {
		return 0, fmt.Errorf("master '%s' not monitored", name)
	}

	peers := s.sentinels[name]
	alive := 0
	for _, p := range peers {
		// Skip self: the "+1" below already counts this sentinel, and
		// gossipSentinels registers self as a peer.
		if p.ID == s.id {
			continue
		}
		if time.Since(p.LastSeen) < s.downAfter {
			alive++
		}
	}

	return alive + 1, nil
}

func (s *Sentinel) Failover(name string) error {
	s.mu.Lock()
	m, ok := s.masters[name]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("master '%s' not monitored", name)
	}

	if m.FailoverState != "" {
		s.mu.Unlock()
		return fmt.Errorf("failover already in progress")
	}
	s.mu.Unlock()

	go s.startFailover(name, m)
	return nil
}

func (s *Sentinel) Reset(pattern string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for name := range s.masters {
		if matchPattern(name, pattern) {
			delete(s.masters, name)
			delete(s.sentinels, name)
			count++
		}
	}

	return count
}

func (s *Sentinel) OnFailover(fn func(master string, newAddr string, newPort int)) {
	s.onFailover = fn
}

func (s *Sentinel) Info() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"sentinel_id":   s.id,
		"sentinel_addr": s.addr,
		"sentinel_port": s.port,
		"masters":       len(s.masters),
		"running":       s.running.Load(),
		"down_after_ms": s.downAfter.Milliseconds(),
		"quorum":        s.quorum,
	}
}

func matchPattern(s, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		return strings.Contains(s, pattern[1:len(pattern)-1])
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(s, pattern[1:])
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(s, pattern[:len(pattern)-1])
	}
	return s == pattern
}

func (s *Sentinel) Serve(ctx context.Context, port int) error {
	addr := net.JoinHostPort(s.addr, strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	logger.Info().Str("addr", addr).Msg("Sentinel listening")

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}

		go s.handleConnection(conn)
	}
}

func (s *Sentinel) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			continue
		}
		response := s.handleCommand(line)
		if _, err := conn.Write([]byte(response + "\r\n")); err != nil {
			return
		}
	}
}

func (s *Sentinel) handleCommand(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "-ERR empty command"
	}

	switch strings.ToUpper(parts[0]) {
	case "PING":
		return "+PONG"
	case "SENTINEL":
		if len(parts) < 2 {
			return "-ERR wrong number of arguments"
		}
		return s.handleSentinel(parts[1:])
	case "INFO":
		return "+OK"
	default:
		return "-ERR unknown command '" + parts[0] + "'"
	}
}

func (s *Sentinel) handleSentinel(parts []string) string {
	if len(parts) == 0 {
		return "-ERR wrong number of arguments"
	}

	switch strings.ToUpper(parts[0]) {
	case "MASTERS":
		return s.formatMasters()
	case "MASTER":
		if len(parts) < 2 {
			return "-ERR wrong number of arguments"
		}
		return s.formatMaster(parts[1])
	case "GETMASTER":
		if len(parts) < 2 {
			return "-ERR wrong number of arguments"
		}
		addr, port, err := s.GetMasterAddr(parts[1])
		if err != nil {
			return "-ERR " + err.Error()
		}
		return fmt.Sprintf("*2\r\n$%d\r\n%s\r\n:%d\r\n", len(addr), addr, port)
	case "HELLO":
		// SENTINEL HELLO <addr> <port> — the peer-announcement command.
		// A sentinel that can reach us tells us who and where it is; we record
		// it against every master we monitor and refresh its LastSeen. This is
		// what lets sentinels discover each other: gossip sends HELLO to a peer,
		// and the receiving side adds the sender to its own table with no manual
		// registration. It replies with our identity so the sender can do the
		// same in one round trip.
		if len(parts) < 3 {
			return "-ERR wrong number of arguments"
		}
		peerPort, err := strconv.Atoi(parts[2])
		if err != nil || peerPort <= 0 || peerPort > 65535 {
			return "-ERR invalid port"
		}
		s.recordPeer(parts[1], peerPort)
		return fmt.Sprintf("+OK %s %s %d", s.id, s.addr, s.port)
	case "RESET":
		if len(parts) < 2 {
			return "-ERR wrong number of arguments"
		}
		count := s.Reset(parts[1])
		return fmt.Sprintf(":%d", count)
	default:
		return "-ERR unknown subcommand '" + parts[0] + "'"
	}
}

func (s *Sentinel) formatMasters() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result strings.Builder
	for _, m := range s.masters {
		fmt.Fprintf(&result, "name:%s\r\nip:%s\r\nport:%d\r\nflags:%s\r\nnum-replicas:%d\r\n",
			m.Name, m.Addr, m.Port, strings.Join(m.Flags, ","), m.NumReplicas)
	}
	return result.String()
}

func (s *Sentinel) formatMaster(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.masters[name]
	if !ok {
		return "-ERR no such master"
	}

	return fmt.Sprintf("*28\r\n"+
		"$4\r\nname\r\n$%d\r\n%s\r\n"+
		"$2\r\nip\r\n$%d\r\n%s\r\n"+
		"$4\r\nport\r\n:%d\r\n"+
		"$5\r\nflags\r\n$%d\r\n%s\r\n",
		len(m.Name), m.Name,
		len(m.Addr), m.Addr,
		m.Port,
		len(strings.Join(m.Flags, ",")), strings.Join(m.Flags, ","))
}
