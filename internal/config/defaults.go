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
			WriteBufferSize: 16384,
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
