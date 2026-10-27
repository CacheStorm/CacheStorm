package server

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/store"
)

func configYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func invalidConfigArgument(name, value string) error {
	return errors.New("ERR CONFIG SET failed (possibly related to argument '" + name + "') - argument couldn't be parsed into an integer or is out of range: '" + value + "'")
}

// configHooks connects CONFIG GET/SET to the running server.
func (s *Server) configHooks() *command.ConfigHooks {
	return &command.ConfigHooks{Values: s.configValues, Set: s.configSet, Rewrite: s.rewriteConfig}
}

func (s *Server) configValues() map[string]string {
	limit, policy := s.store.MemoryLimit()
	samples := s.cfg.Memory.SampleSize
	if ev := s.store.Evictor(); ev != nil {
		samples = ev.SampleSize()
	}
	appendfsync := s.cfg.Persistence.AOFSync
	if appendfsync == "" {
		appendfsync = "everysec"
	}
	return map[string]string{
		"maxmemory":                   strconv.FormatInt(limit, 10),
		"maxmemory-policy":            policy.String(),
		"maxmemory-samples":           strconv.Itoa(samples),
		"maxclients":                  strconv.FormatInt(s.maxClients.Load(), 10),
		"timeout":                     strconv.FormatInt(int64(time.Duration(s.idleTimeout.Load())/time.Second), 10),
		"port":                        strconv.Itoa(s.ListeningPort()),
		"bind":                        s.cfg.Server.Bind,
		"databases":                   strconv.Itoa(s.dbs.Count()),
		"protected-mode":              configYesNo(s.cfg.Server.ProtectedMode),
		"tcp-keepalive":               strconv.Itoa(s.cfg.Server.TCPKeepAlive),
		"requirepass":                 s.router.RequirePass(),
		"appendonly":                  configYesNo(s.aof != nil),
		"appendfsync":                 appendfsync,
		"appendfilename":              "appendonly.aof",
		"dbfilename":                  "dump.rdb",
		"dir":                         s.persistenceDirectory(),
		"auto-aof-rewrite-percentage": strconv.FormatInt(s.aofRewritePercentage, 10),
		"auto-aof-rewrite-min-size":   strconv.FormatInt(s.aofRewriteMinSize, 10),
	}
}

func (s *Server) configSet(name, value string) (bool, error) {
	switch name {
	case "maxmemory":
		limit, err := config.ParseMemorySize(value)
		if err != nil || limit < 0 {
			return true, invalidConfigArgument(name, value)
		}
		_, policy := s.store.MemoryLimit()
		s.store.SetMemoryLimit(limit, policy, 0)
		return true, nil
	case "maxmemory-policy":
		policy, ok := store.ParseEvictionPolicy(strings.ToLower(value))
		if !ok {
			return true, errors.New("ERR CONFIG SET failed (possibly related to argument 'maxmemory-policy') - argument(s) must be one of the following: volatile-lru, allkeys-lru, volatile-lfu, allkeys-lfu, volatile-random, allkeys-random, volatile-ttl, noeviction")
		}
		limit, _ := s.store.MemoryLimit()
		s.store.SetMemoryLimit(limit, policy, 0)
		return true, nil
	case "maxmemory-samples":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 64 {
			return true, invalidConfigArgument(name, value)
		}
		limit, policy := s.store.MemoryLimit()
		s.store.SetMemoryLimit(limit, policy, n)
		return true, nil
	case "maxclients":
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n < 1 {
			return true, invalidConfigArgument(name, value)
		}
		s.maxClients.Store(n)
		return true, nil
	case "timeout":
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n < 0 || n > int64(time.Duration(1<<62)/time.Second) {
			return true, invalidConfigArgument(name, value)
		}
		s.idleTimeout.Store(int64(time.Duration(n) * time.Second))
		return true, nil
	case "requirepass":
		s.router.SetRequirePass(value)
		return true, nil
	case "port", "bind", "databases", "dir", "dbfilename", "appendfilename", "appendonly", "protected-mode", "tcp-keepalive", "appendfsync":
		return true, errors.New("ERR CONFIG SET failed (possibly related to argument '" + name + "') - can't set immutable config")
	}
	return false, nil
}
