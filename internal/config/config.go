package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server      ServerConfig               `yaml:"server"`
	HTTP        HTTPConfig                 `yaml:"http"`
	Memory      MemoryConfig               `yaml:"memory"`
	Namespaces  map[string]NamespaceConfig `yaml:"namespaces"`
	Cluster     ClusterConfig              `yaml:"cluster"`
	Persistence PersistenceConfig          `yaml:"persistence"`
	Replication ReplicationConfig          `yaml:"replication"`
	Plugins     PluginsConfig              `yaml:"plugins"`
	Logging     LoggingConfig              `yaml:"logging"`
}

type ServerConfig struct {
	Bind            string `yaml:"bind" default:"0.0.0.0"`
	Port            int    `yaml:"port" default:"6380"`
	MaxConnections  int    `yaml:"max_connections" default:"10000"`
	TCPKeepAlive    int    `yaml:"tcp_keepalive" default:"300"`
	ReadTimeout     string `yaml:"read_timeout" default:"0"`
	WriteTimeout    string `yaml:"write_timeout" default:"0"`
	ReadBufferSize  int    `yaml:"read_buffer_size" default:"4096"`
	WriteBufferSize int    `yaml:"write_buffer_size" default:"4096"`
}

type HTTPConfig struct {
	Enabled  bool   `yaml:"enabled" default:"true"`
	Port     int    `yaml:"port" default:"8080"`
	Password string `yaml:"password"`
}

type MemoryConfig struct {
	MaxMemory      string `yaml:"max_memory" default:"0"`
	EvictionPolicy string `yaml:"eviction_policy" default:"allkeys-lru"`
	WarningPct     int    `yaml:"pressure_warning" default:"70"`
	CriticalPct    int    `yaml:"pressure_critical" default:"85"`
	SampleSize     int    `yaml:"eviction_sample_size" default:"5"`
}

type NamespaceConfig struct {
	DefaultTTL string `yaml:"default_ttl" default:"0"`
	MaxMemory  string `yaml:"max_memory" default:"0"`
}

type ClusterConfig struct {
	Enabled       bool     `yaml:"enabled" default:"false"`
	NodeName      string   `yaml:"node_name"`
	BindAddr      string   `yaml:"bind_addr" default:"0.0.0.0"`
	BindPort      int      `yaml:"bind_port" default:"7946"`
	AdvertiseAddr string   `yaml:"advertise_addr"`
	AdvertisePort int      `yaml:"advertise_port"`
	Seeds         []string `yaml:"seeds"`
	Replicas      int      `yaml:"replicas" default:"1"`
}

type PersistenceConfig struct {
	Enabled          bool   `yaml:"enabled" default:"false"`
	AOF              bool   `yaml:"aof" default:"true"`
	AOFSync          string `yaml:"aof_sync" default:"everysec"`
	SnapshotInterval string `yaml:"snapshot_interval" default:"5m"`
	DataDir          string `yaml:"data_dir" default:"/var/lib/cachestorm"`
	MaxAOFSize       string `yaml:"max_aof_size" default:"1gb"`
}

type ReplicationConfig struct {
	Role                string `yaml:"role" default:"master"`
	MasterHost          string `yaml:"master_host"`
	MasterPort          int    `yaml:"master_port"`
	MasterAuth          string `yaml:"master_auth"`
	ReplicaAnnounceIP   string `yaml:"replica_announce_ip"`
	ReplicaAnnouncePort int    `yaml:"replica_announce_port"`
	ReadOnly            bool   `yaml:"read_only" default:"true"`
	ReplTimeout         int    `yaml:"repl_timeout" default:"60"`
}

type PluginsConfig struct {
	Stats   StatsPluginConfig   `yaml:"stats"`
	Metrics MetricsPluginConfig `yaml:"metrics"`
	Auth    AuthPluginConfig    `yaml:"auth"`
	SlowLog SlowLogPluginConfig `yaml:"slowlog"`
}

type StatsPluginConfig struct {
	Enabled bool `yaml:"enabled" default:"true"`
}

type MetricsPluginConfig struct {
	Enabled bool   `yaml:"enabled" default:"true"`
	Port    int    `yaml:"port" default:"9090"`
	Path    string `yaml:"path" default:"/metrics"`
}

type AuthPluginConfig struct {
	Enabled  bool   `yaml:"enabled" default:"false"`
	Password string `yaml:"password"`
}

type SlowLogPluginConfig struct {
	Enabled    bool   `yaml:"enabled" default:"true"`
	Threshold  string `yaml:"threshold" default:"10ms"`
	MaxEntries int    `yaml:"max_entries" default:"1000"`
}

type LoggingConfig struct {
	Level  string `yaml:"level" default:"info"`
	Format string `yaml:"format" default:"json"`
	Output string `yaml:"output" default:"stdout"`
}

func Load(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		path = os.Getenv("CACHESTORM_CONFIG")
	}

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	cfg.applyEnvOverrides()

	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("CACHESTORM_BIND"); v != "" {
		c.Server.Bind = v
	}
	if v := os.Getenv("CACHESTORM_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.Port = port
		}
	}
	if v := os.Getenv("CACHESTORM_MAX_MEMORY"); v != "" {
		c.Memory.MaxMemory = v
	}
	if v := os.Getenv("CACHESTORM_LOG_LEVEL"); v != "" {
		c.Logging.Level = v
	}
}

func (c *ServerConfig) ReadTimeoutDuration() time.Duration {
	d, err := time.ParseDuration(c.ReadTimeout)
	if err != nil {
		return 0
	}
	return d
}

func (c *ServerConfig) WriteTimeoutDuration() time.Duration {
	d, err := time.ParseDuration(c.WriteTimeout)
	if err != nil {
		return 0
	}
	return d
}
