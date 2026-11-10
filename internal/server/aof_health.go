package server

import (
	"errors"

	"github.com/cachestorm/cachestorm/internal/logger"
)

var errAOFUnavailable = errors.New("MISCONF AOF persistence is unavailable; writes are disabled until restart")

func (s *Server) persistenceError() error {
	if aof := s.aof(); aof != nil && aof.Err() != nil {
		return errAOFUnavailable
	}
	return nil
}

// writeOutAOF hands the appended AOF to the kernel before replies go out;
// see AOFWriter.WriteOut. A failure is kept by the writer, which then
// refuses further writes with MISCONF, as Redis does after a failed AOF
// write.
func (s *Server) writeOutAOF() {
	if aof := s.aof(); aof != nil {
		if err := aof.WriteOut(); err != nil {
			logger.Debug().Err(err).Msg("AOF write before replies failed")
		}
	}
}
