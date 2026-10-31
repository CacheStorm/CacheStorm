package server

import (
	"strconv"
	"time"

	"github.com/cachestorm/cachestorm/internal/cluster"
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
	replicaof := ""
	if s.repl != nil {
		if host, port, ok := s.repl.MasterAddr(); ok {
			replicaof = host + " " + strconv.Itoa(port)
		}
	}
	rc := s.cfg.Replication
	values := map[string]string{
		"cluster-enabled":             configYesNo(s.cfg.Cluster.Enabled),
		"replicaof":                   replicaof,
		"masterauth":                  rc.MasterAuth,
		"repl-backlog-size":           strconv.Itoa(rc.BacklogSize),
		"repl-timeout":                strconv.Itoa(rc.ReplTimeout),
		"replica-read-only":           configYesNo(rc.ReadOnly),
		"replica-announce-ip":         rc.ReplicaAnnounceIP,
		"replica-announce-port":       strconv.Itoa(rc.ReplicaAnnouncePort),
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
		"auto-aof-rewrite-percentage": strconv.FormatInt(s.aofRewritePercentage.Load(), 10),
		"auto-aof-rewrite-min-size":   strconv.FormatInt(s.aofRewriteMinSize.Load(), 10),
		"save":                        s.snapshotRules(),
	}
	if c := s.router.Cluster(); c != nil {
		cc, opts := s.cfg.Cluster, c.Options()
		values["cluster-config-file"] = cc.ConfigFile
		values["cluster-node-timeout"] = strconv.Itoa(cc.NodeTimeout)
		values["cluster-port"] = strconv.Itoa(cc.BusPort)
		values["cluster-announce-ip"] = cc.AnnounceIP
		values["cluster-announce-port"] = strconv.Itoa(cc.AnnouncePort)
		values["cluster-announce-bus-port"] = strconv.Itoa(cc.AnnounceBusPort)
		values["cluster-replica-validity-factor"] = strconv.Itoa(cc.ReplicaValidityFactor)
		values["cluster-require-full-coverage"] = configYesNo(opts.RequireFullCoverage)
		values["cluster-allow-reads-when-down"] = configYesNo(opts.AllowReadsWhenDown)
		values["cluster-replica-no-failover"] = configYesNo(opts.ReplicaNoFailover)
		values["cluster-allow-replica-migration"] = configYesNo(opts.AllowReplicaMigration)
		values["cluster-migration-barrier"] = strconv.Itoa(opts.MigrationBarrier)
	}
	return values
}

func (s *Server) configSet(name, value string) (bool, error) {
	switch name {
	case "maxmemory":
		limit, err := config.ParseMemorySize(value)
		if err != nil || limit < 0 {
			return true, command.ConfigSetError(name, "argument must be a memory value")
		}
		_, policy := s.store.MemoryLimit()
		s.store.SetMemoryLimit(limit, policy, 0)
		applyGoMemoryLimit(limit)
		return true, nil
	case "maxmemory-policy":
		v, err := command.ParseConfigEnum(name, value, "volatile-lru", "volatile-lfu", "volatile-random", "volatile-ttl",
			"allkeys-lru", "allkeys-lfu", "allkeys-random", "noeviction")
		if err != nil {
			return true, err
		}
		policy, _ := store.ParseEvictionPolicy(v)
		limit, _ := s.store.MemoryLimit()
		s.store.SetMemoryLimit(limit, policy, 0)
		return true, nil
	case "auto-aof-rewrite-percentage":
		n, err := command.ParseConfigInt(name, value, 0, 1<<31-1)
		if err != nil {
			return true, err
		}
		s.aofRewritePercentage.Store(n)
		return true, nil
	case "auto-aof-rewrite-min-size":
		size, err := config.ParseMemorySize(value)
		if err != nil || size < 0 {
			return true, command.ConfigSetError(name, "argument must be a memory value")
		}
		s.aofRewriteMinSize.Store(size)
		return true, nil
	case "maxmemory-samples":
		n, err := command.ParseConfigInt(name, value, 1, 64)
		if err != nil {
			return true, err
		}
		limit, policy := s.store.MemoryLimit()
		s.store.SetMemoryLimit(limit, policy, int(n))
		return true, nil
	case "maxclients":
		n, err := command.ParseConfigInt(name, value, 1, 1<<32-1)
		if err != nil {
			return true, err
		}
		s.maxClients.Store(n)
		return true, nil
	case "timeout":
		n, err := command.ParseConfigInt(name, value, 0, 1<<31-1)
		if err != nil {
			return true, err
		}
		s.idleTimeout.Store(int64(time.Duration(n) * time.Second))
		return true, nil
	case "requirepass":
		s.router.SetRequirePass(value)
		return true, nil
	case "port", "tcp-keepalive":
		hi := int64(1<<31 - 1)
		if name == "port" {
			hi = 65535
		}
		if _, err := command.ParseConfigInt(name, value, 0, hi); err != nil {
			return true, err
		}
		return true, command.ConfigSetError(name, "can't set immutable config")
	case "appendonly", "protected-mode":
		if _, err := command.ParseConfigBool(name, value); err != nil {
			return true, err
		}
		return true, command.ConfigSetError(name, "can't set immutable config")
	case "appendfsync":
		if _, err := command.ParseConfigEnum(name, value, "everysec", "always", "no"); err != nil {
			return true, err
		}
		return true, command.ConfigSetError(name, "can't set immutable config")
	case "dir", "dbfilename":
		return true, command.ConfigSetError(name, "can't set protected config")
	case "masterauth", "repl-backlog-size", "repl-timeout", "replica-read-only", "replica-announce-ip", "replica-announce-port":
		// The replication settings are read when replication starts.
		if err := command.ValidateConfigParam(name, value); err != nil {
			return true, err
		}
		return true, command.ConfigSetError(name, "can't set immutable config")
	case "cluster-require-full-coverage", "cluster-allow-reads-when-down", "cluster-replica-no-failover",
		"cluster-allow-replica-migration", "cluster-migration-barrier", "cluster-config-file", "cluster-node-timeout",
		"cluster-port", "cluster-announce-ip", "cluster-announce-port", "cluster-announce-bus-port",
		"cluster-replica-validity-factor":
		// Without cluster mode these are plain stored parameters.
		c := s.router.Cluster()
		if c == nil {
			return false, nil
		}
		if err := command.ValidateConfigParam(name, value); err != nil {
			return true, err
		}
		var update func(*cluster.Options)
		switch name {
		case "cluster-migration-barrier":
			n, err := command.ParseConfigInt(name, value, 0, 1<<31-1)
			if err != nil {
				return true, err
			}
			update = func(o *cluster.Options) { o.MigrationBarrier = int(n) }
		case "cluster-require-full-coverage", "cluster-allow-reads-when-down", "cluster-replica-no-failover", "cluster-allow-replica-migration":
			yes, err := command.ParseConfigBool(name, value)
			if err != nil {
				return true, err
			}
			update = func(o *cluster.Options) {
				switch name {
				case "cluster-require-full-coverage":
					o.RequireFullCoverage = yes
				case "cluster-allow-reads-when-down":
					o.AllowReadsWhenDown = yes
				case "cluster-replica-no-failover":
					o.ReplicaNoFailover = yes
				default:
					o.AllowReplicaMigration = yes
				}
			}
		default:
			// The bus reads these when it starts.
			return true, command.ConfigSetError(name, "can't set immutable config")
		}
		c.UpdateOptions(update)
		return true, nil
	case "save":
		return true, s.setSnapshotRules(value)
	case "bind", "databases", "appendfilename":
		return true, command.ConfigSetError(name, "can't set immutable config")
	}
	return false, nil
}
