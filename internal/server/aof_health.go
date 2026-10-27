package server

import "errors"

var errAOFUnavailable = errors.New("MISCONF AOF persistence is unavailable; writes are disabled until restart")

func (s *Server) persistenceError() error {
	if s.aof != nil && s.aof.Err() != nil {
		return errAOFUnavailable
	}
	return nil
}
