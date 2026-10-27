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
