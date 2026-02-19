package config

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Bind:            "0.0.0.0",
			Port:            6380,
			MaxConnections:  10000,
			TCPKeepAlive:    300,
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
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
		Persistence: PersistenceConfig{
			AOF:              true,
			AOFSync:          "everysec",
			SnapshotInterval: "5m",
			DataDir:          "/var/lib/cachestorm",
			MaxAOFSize:       "1gb",
		},
		Plugins: PluginsConfig{
			Stats: StatsPluginConfig{
				Enabled: true,
			},
			Metrics: MetricsPluginConfig{
				Enabled: true,
				Port:    9090,
				Path:    "/metrics",
			},
			SlowLog: SlowLogPluginConfig{
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
	}
}
