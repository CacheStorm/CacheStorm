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
			WriteBufferSize: 4096,
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
		Namespaces: map[string]NamespaceConfig{
			"default": {},
		},
		Cluster: ClusterConfig{
			BindPort: 7946,
			Replicas: 1,
		},
		Replication: ReplicationConfig{
			Role:               "master",
			ReadOnly:           true,
			ReplTimeout:        60,
			BacklogSize:        1 << 20,
			ReplicaBufferLimit: 64 << 20,
		},
		Persistence: PersistenceConfig{
			AOF:              true,
			AOFSync:          "everysec",
			SnapshotInterval: "5m",
			DataDir:          "/var/lib/cachestorm",
			MaxAOFSize:       "1gb",
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
