package server

func (s *Server) persistenceDirectory() string {
	if s.dataDirectory != "" {
		return s.dataDirectory
	}
	if s.cfg.Persistence.DataDir != "" {
		return s.cfg.Persistence.DataDir
	}
	return "."
}
