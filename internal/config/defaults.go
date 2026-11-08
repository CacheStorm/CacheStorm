package config

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Bind:            "0.0.0.0",
			Port:            6380,
			MaxConnections:  10000,
			Databases:       16,
			TCPKeepAlive:    300,
			ReadBufferSize:  4096,
			WriteBufferSize: 512,
			ProtectedMode:   true,
		},
		HTTP: HTTPConfig{
			Enabled:    true,
			Port:       8080,
			Password:   "",
			RateLimit:  100,
			RateWindow: "1m",
		},
		Memory: MemoryConfig{
			MaxMemory:      "0",
			EvictionPolicy: "allkeys-lru",
			WarningPct:     70,
			CriticalPct:    85,
			SampleSize:     5,
		},
		Cluster: ClusterConfig{
			ConfigFile:            "nodes.conf",
			NodeTimeout:           15000,
			RequireFullCoverage:   true,
			ReplicaValidityFactor: 10,
			AllowReplicaMigration: true,
			MigrationBarrier:      1,
		},
		Replication: ReplicationConfig{
			Role:               "master",
			ReadOnly:           true,
			ReplTimeout:        60,
			BacklogSize:        1 << 20,
			ReplicaBufferLimit: 64 << 20,
		},
		Persistence: PersistenceConfig{
			AOF:                  true,
			AOFSync:              "everysec",
			SnapshotInterval:     "5m",
			DataDir:              "/var/lib/cachestorm",
			AOFRewritePercentage: 100,
			AOFRewriteMinSize:    "64mb",
		},
		Plugins: PluginsConfig{
			Metrics: MetricsPluginConfig{
				Enabled: true,
				Port:    9090,
				Path:    "/metrics",
			},
			SlowLog: SlowLogConfig{
				Enabled:    true,
				Threshold:  "10ms",
				MaxEntries: 1000,
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Sentinel: SentinelConfig{
			ID:           "sentinel-1",
			Addr:         "127.0.0.1",
			Port:         26379,
			Quorum:       2,
			DownAfter:    "30s",
			FailoverTime: "3m",
		},
	}
}

// UseRedisDefaults sets what redis-server does where CacheStorm's own
// defaults differ, for a server started from a redis.conf: it listens on
// its RESP port only, without the HTTP API or the metrics endpoint (the
// http-port and metrics-port directives turn them on), and refuses writes
// over maxmemory instead of evicting unless maxmemory-policy says otherwise.
func UseRedisDefaults(c *Config) {
	c.HTTP.Enabled = false
	c.Plugins.Metrics.Enabled = false
	c.Memory.EvictionPolicy = "noeviction"
}
