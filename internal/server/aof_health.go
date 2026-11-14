package server

import (
	"errors"

	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/persistence"
)

var errAOFUnavailable = errors.New("MISCONF AOF persistence is unavailable; writes are disabled until restart")

func (s *Server) persistenceError() error {
	if aof := s.aof(); aof != nil && aof.Err() != nil {
		return errAOFUnavailable
	}
	return nil
}

// commitAOF finishes an append under appendfsync always: it writes and
// fsyncs the AOF before the command's reply is made, unless the command is
// followed by more of its pipeline (deferred), whose replies go out after
// one fsync covering them all. A failure makes the command's reply MISCONF.
func (s *Server) commitAOF(aof *persistence.AOFWriter, deferred bool) error {
	if deferred || !aof.SyncsAlways() {
		return nil
	}
	if err := aof.WriteOut(); err != nil {
		logger.Error().Err(err).Msg("AOF fsync failed")
		return errAOFUnavailable
	}
	return nil
}

// syncRemoval finishes the append of a removal no reply waits for (an
// expiry, a slot handed away). A failure is logged and kept by the writer.
func (s *Server) syncRemoval(aof *persistence.AOFWriter) {
	if err := s.commitAOF(aof, false); err != nil {
		logger.Debug().Err(err).Msg("AOF fsync of a removal failed")
	}
}

// flushAOFForReply is writeOutAOF for a reply with no deferred writes (an
// HTTP response, whose writes fsynced inline). A failure is logged and
// kept by the writer.
func (s *Server) flushAOFForReply() {
	if err := s.writeOutAOF(); err != nil {
		logger.Debug().Err(err).Msg("AOF write before an HTTP reply failed")
	}
}

// writeOutAOF hands the appended AOF to the kernel before replies go out,
// and under appendfsync always fsyncs it; see AOFWriter.WriteOut. A failure
// is kept by the writer, which then refuses further writes with MISCONF, as
// Redis does after a failed AOF write. The error tells a connection to
// turn the replies of its deferred writes into MISCONF.
func (s *Server) writeOutAOF() error {
	aof := s.aof()
	if aof == nil {
		return nil
	}
	err := aof.WriteOut()
	if err != nil {
		logger.Debug().Err(err).Msg("AOF write before replies failed")
	}
	return err
}
