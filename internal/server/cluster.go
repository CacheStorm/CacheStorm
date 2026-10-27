package server

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

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

// announceCluster records the address this node announces once the
// listener is up, and saves it.
func (s *Server) announceCluster() error {
	c := s.router.Cluster()
	if c == nil {
		return nil
	}
	cc := s.cfg.Cluster
	port := s.ListeningPort()
	if cc.AnnouncePort != 0 {
		port = cc.AnnouncePort
	}
	cport := cc.BusPort
	if cport == 0 {
		cport = s.ListeningPort() + 10000
	}
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
	return c.Save()
}
