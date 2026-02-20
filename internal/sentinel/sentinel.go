package sentinel

import (
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
	}
}

func (s *Sentinel) Start() error {
	s.running.Store(true)

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

			if s.checkODown(master) {
				master.State = MasterStateODown
				master.Flags = []string{"master", "s_down", "o_down"}
				logger.Error().
					Str("master", name).
					Msg("Master marked as objectively down")

				go s.startFailover(name, master)
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

	downCount := 0
	for _, peer := range peers {
		if time.Since(peer.LastSeen) < s.downAfter {
			downCount++
		}
	}

	return downCount+1 >= s.quorum
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

	time.Sleep(5 * time.Second)

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

func (s *Sentinel) gossipSentinels() {
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

	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		data := string(buf[:n])
		lines := strings.Split(data, "\r\n")

		for _, line := range lines {
			if line == "" {
				continue
			}

			response := s.handleCommand(line)
			conn.Write([]byte(response + "\r\n"))
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
		result.WriteString(fmt.Sprintf("name:%s\r\nip:%s\r\nport:%d\r\nflags:%s\r\nnum-replicas:%d\r\n",
			m.Name, m.Addr, m.Port, strings.Join(m.Flags, ","), m.NumReplicas))
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
