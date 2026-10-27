package server

import (
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/store"
)

func (s *Server) recordRemoval(st *store.Store, key string) {
	dbIndex := -1
	for db := 0; db < s.dbs.Count(); db++ {
		if s.dbs.Existing(db) == st {
			dbIndex = db
			break
		}
	}
	if dbIndex < 0 {
		return
	}
	if s.tracker != nil && s.tracker.active() {
		s.tracker.invalidate(0, []string{key})
	}
	if s.repl != nil {
		s.repl.MarkLocalWrite()
	}
	if s.aof != nil {
		if err := s.aof.AppendDB(dbIndex, "DEL", [][]byte{[]byte(key)}); err != nil {
			logger.Error().Err(err).Msg("AOF removal append failed")
		}
	}
	if s.repl != nil {
		s.repl.FeedDelete(st, key)
	}
}

// recordFieldRemoval records the expiry of hash fields as HDEL in the AOF
// and the replication stream, and invalidates tracking clients' caches of
// the key. It is called with the key's shard lock held.
func (s *Server) recordFieldRemoval(st *store.Store, key string, fields []string) {
	dbIndex := -1
	for db := 0; db < s.dbs.Count(); db++ {
		if s.dbs.Existing(db) == st {
			dbIndex = db
			break
		}
	}
	if dbIndex < 0 {
		return
	}
	if s.tracker != nil && s.tracker.active() {
		s.tracker.invalidate(0, []string{key})
	}
	if s.repl != nil {
		s.repl.MarkLocalWrite()
	}
	if s.aof != nil {
		for _, f := range fields {
			if err := s.aof.AppendDB(dbIndex, "HDEL", [][]byte{[]byte(key), []byte(f)}); err != nil {
				logger.Error().Err(err).Msg("AOF field expiry append failed")
			}
		}
	}
	if s.repl != nil {
		s.repl.FeedHashFieldDeletes(st, key, fields)
	}
}

// withFieldExpiryHooks adds the propagation of hash field expiry to h.
func (s *Server) withFieldExpiryHooks(h store.StoreHooks) store.StoreHooks {
	h.OnHashFieldsExpiredLocked = s.recordFieldRemoval
	return h
}
