package server

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cachestorm/cachestorm/internal/cluster"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/store"
)

// errClusterDisabled is the HTTP API's answer to a cluster request on a
// server running without cluster mode.
var errClusterDisabled = errors.New("cluster mode is not enabled: set cluster.enabled to true")

// openCluster loads or creates the node's nodes.conf and turns cluster mode
// on. It runs before any data is loaded, so the per-slot key index sees
// every key.
func (s *Server) openCluster() error {
	cc := s.cfg.Cluster
	dir := s.cfg.Persistence.DataDir
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	c, created, err := cluster.Open(filepath.Join(dir, cc.ConfigFile), cluster.Options{
		RequireFullCoverage: cc.RequireFullCoverage,
		AllowReadsWhenDown:  cc.AllowReadsWhenDown,
	})
	if err != nil {
		return err
	}
	if created {
		logger.Info().Str("id", c.MyID()).Msg("cluster: no nodes.conf found, created a new node")
	} else {
		logger.Info().Str("id", c.MyID()).Msg("cluster: node configuration loaded")
	}
	s.store.EnableSlotIndex()
	s.router.SetCluster(c)
	return nil
}

// verifyClusterData is Redis' verifyClusterConfigWithData: keys may live
// only in database 0, and a slot holding keys that no node serves becomes
// this node's.
func (s *Server) verifyClusterData() error {
	c := s.router.Cluster()
	if c == nil {
		return nil
	}
	var bad error
	s.dbs.ForEach(func(index int, db *store.Store) {
		if index != 0 && db.KeyCount() > 0 && bad == nil {
			bad = fmt.Errorf("database %d holds keys: cluster mode serves only database 0; remove the snapshot and AOF or start without cluster mode", index)
		}
	})
	if bad != nil {
		return bad
	}
	if s.store.KeyCount() == 0 {
		return nil
	}
	for slot := 0; slot < cluster.Slots; slot++ {
		if s.store.CountKeysInSlot(slot) > 0 {
			c.ClaimSlot(slot)
		}
	}
	return c.Save()
}

// announceCluster opens the cluster bus listener, records the address this
// node announces once the client listener is up, saves it and starts the
// bus. With an ephemeral client port and no bus_port the bus port is
// ephemeral too.
func (s *Server) announceCluster() error {
	c := s.router.Cluster()
	if c == nil {
		return nil
	}
	cc := s.cfg.Cluster
	busPort := cc.BusPort
	if busPort == 0 && s.cfg.Server.Port != 0 {
		busPort = s.ListeningPort() + 10000
	}
	if busPort > 65535 {
		return fmt.Errorf("cluster bus port %d is out of range: set cluster.bus_port", busPort)
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(s.cfg.Server.Bind, strconv.Itoa(busPort)))
	if err != nil {
		return fmt.Errorf("cluster bus: %w", err)
	}
	port := s.ListeningPort()
	if cc.AnnouncePort != 0 {
		port = cc.AnnouncePort
	}
	cport := ln.Addr().(*net.TCPAddr).Port
	if cc.AnnounceBusPort != 0 {
		cport = cc.AnnounceBusPort
	}
	ip := cc.AnnounceIP
	if ip == "" {
		if bind := net.ParseIP(s.cfg.Server.Bind); bind != nil && !bind.IsUnspecified() {
			ip = bind.String()
		}
	}
	c.SetMyself(ip, port, cport, 0, "")
	if err := c.Save(); err != nil {
		ln.Close()
		return err
	}
	ps := s.store.GetPubSub()
	if err := c.StartBus(cluster.BusOptions{
		Listener:      ln,
		NodeTimeout:   time.Duration(cc.NodeTimeout) * time.Millisecond,
		Publish:       func(channel string, msg []byte) { ps.Publish(channel, msg) },
		KeysInSlot:    s.store.CountKeysInSlot,
		DelKeysInSlot: s.dropSlotKeys,
	}); err != nil {
		ln.Close()
		return err
	}
	logger.Info().Str("addr", ln.Addr().String()).Msg("cluster bus started")
	return nil
}

// stopClusterBus stops the cluster bus and its goroutines.
func (s *Server) stopClusterBus() {
	if c := s.router.Cluster(); c != nil {
		c.StopBus()
	}
}

// dropSlotKeys deletes the keys of a slot another node took over, as Redis'
// delKeysInSlot does, and records the deletions in the AOF and the
// replication stream.
func (s *Server) dropSlotKeys(slot int) {
	for {
		keys := s.store.KeysInSlot(slot, 1000)
		if len(keys) == 0 {
			return
		}
		args := make([][]byte, 0, len(keys))
		for _, k := range keys {
			if s.store.Delete(k) {
				args = append(args, []byte(k))
			}
		}
		if len(args) == 0 {
			return
		}
		if s.aof != nil {
			if err := s.aof.AppendDB(0, "DEL", args); err != nil {
				logger.Error().Err(err).Msg("AOF append failed")
			}
		}
		s.repl.Feed(0, "DEL", args)
	}
}
