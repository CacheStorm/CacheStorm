package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	HTTP        HTTPConfig        `yaml:"http"`
	Memory      MemoryConfig      `yaml:"memory"`
	Cluster     ClusterConfig     `yaml:"cluster"`
	Persistence PersistenceConfig `yaml:"persistence"`
	Replication ReplicationConfig `yaml:"replication"`
	Plugins     PluginsConfig     `yaml:"plugins"`
	Logging     LoggingConfig     `yaml:"logging"`
	Sentinel    SentinelConfig    `yaml:"sentinel"`
	// SourceFile is the file the configuration was loaded from, if any.
	SourceFile string `yaml:"-"`
}

// SentinelConfig configures the Sentinel subsystem wired up by
// command.EnsureSentinel. Durations are strings so YAML can carry "30s"; use
// the DownAfterDuration/FailoverTimeDuration accessors to read them.
type SentinelConfig struct {
	// Enabled starts the Sentinel monitor and its peer listener. Sentinel is a
	// separate role, as in Redis; a data server does not open the sentinel
	// port unless asked to.
	Enabled      bool   `yaml:"enabled" default:"false"`
	ID           string `yaml:"id" default:"sentinel-1"`
	Addr         string `yaml:"addr" default:"127.0.0.1"`
	Port         int    `yaml:"port" default:"26379"`
	Quorum       int    `yaml:"quorum" default:"2"`
	DownAfter    string `yaml:"down_after" default:"30s"`
	FailoverTime string `yaml:"failover_time" default:"3m"`
	// Seeds are "addr:port" addresses of other sentinels to announce ourselves
	// to on each gossip tick. They are how HELLO discovery bootstraps: a
	// sentinel has no other way to learn that peers exist, and knowledge then
	// spreads because every peer we reach records us in turn. Default is empty
	// (no peers), which is correct for a single-sentinel deployment.
	Seeds []string `yaml:"seeds"`
}

// DownAfterDuration parses DownAfter, falling back to 0 when unset or
// unparseable so the Sentinel applies its own default.
func (c *SentinelConfig) DownAfterDuration() time.Duration {
	d, err := time.ParseDuration(c.DownAfter)
	if err != nil {
		return 0
	}
	return d
}

// FailoverTimeDuration parses FailoverTime, falling back to 0 when unset or
// unparseable.
func (c *SentinelConfig) FailoverTimeDuration() time.Duration {
	d, err := time.ParseDuration(c.FailoverTime)
	if err != nil {
		return 0
	}
	return d
}

type ServerConfig struct {
	Bind            string `yaml:"bind" default:"0.0.0.0"`
	Port            int    `yaml:"port" default:"6380"`
	MaxConnections  int    `yaml:"max_connections" default:"10000"`
	Databases       int    `yaml:"databases" default:"16"`
	TCPKeepAlive    int    `yaml:"tcp_keepalive" default:"300"`
	ReadTimeout     string `yaml:"read_timeout" default:"0"`
	WriteTimeout    string `yaml:"write_timeout" default:"0"`
	ReadBufferSize  int    `yaml:"read_buffer_size" default:"4096"`
	WriteBufferSize int    `yaml:"write_buffer_size" default:"16384"`
	RequirePass     string `yaml:"requirepass"`
	// ProtectedMode refuses clients that do not connect over loopback while
	// no password is set, as Redis' protected-mode does, so a server started
	// with the defaults is not an open database on every interface.
	ProtectedMode bool   `yaml:"protected_mode" default:"true"`
	TLSCertFile   string `yaml:"tls_cert_file"`
	TLSKeyFile    string `yaml:"tls_key_file"`
	// TLSCACertFile verifies client certificates and, for a replica, the
	// master's certificate (tls-ca-cert-file).
	TLSCACertFile string `yaml:"tls_ca_cert_file"`
	// TLSAuthClients is tls-auth-clients: "yes" requires a client
	// certificate signed by TLSCACertFile, "optional" verifies one when it
	// is sent, "no" never asks. Empty means "yes" when a CA is configured.
	TLSAuthClients string `yaml:"tls_auth_clients"`
	// NotifyKeyspaceEvents is notify-keyspace-events: the classes of
	// keyspace notifications published over Pub/Sub; empty disables them.
	NotifyKeyspaceEvents string `yaml:"notify_keyspace_events"`
}

type HTTPConfig struct {
	Enabled bool `yaml:"enabled" default:"true"`
	// Bind is the address the HTTP API listens on; empty means the same
	// address as the RESP listener (server.bind).
	Bind       string `yaml:"bind"`
	Port       int    `yaml:"port" default:"8080"`
	Password   string `yaml:"password"`
	RateLimit  int    `yaml:"rate_limit" default:"100"`
	RateWindow string `yaml:"rate_window" default:"1m"`
	CORSOrigin string `yaml:"cors_origin"`
}

type MemoryConfig struct {
	MaxMemory      string `yaml:"max_memory" default:"0"`
	EvictionPolicy string `yaml:"eviction_policy" default:"allkeys-lru"`
	WarningPct     int    `yaml:"pressure_warning" default:"70"`
	CriticalPct    int    `yaml:"pressure_critical" default:"85"`
	SampleSize     int    `yaml:"eviction_sample_size" default:"5"`
}

type ClusterConfig struct {
	Enabled bool `yaml:"enabled" default:"false"`
	// ConfigFile is the nodes.conf file, relative to persistence.data_dir.
	ConfigFile string `yaml:"config_file" default:"nodes.conf"`
	// NodeTimeout is cluster-node-timeout, in milliseconds.
	NodeTimeout int `yaml:"node_timeout" default:"15000"`
	// BusPort is the cluster bus port; 0 means the client port + 10000.
	BusPort             int    `yaml:"bus_port" default:"0"`
	AnnounceIP          string `yaml:"announce_ip"`
	AnnouncePort        int    `yaml:"announce_port"`
	AnnounceBusPort     int    `yaml:"announce_bus_port"`
	RequireFullCoverage bool   `yaml:"require_full_coverage" default:"true"`
	AllowReadsWhenDown  bool   `yaml:"allow_reads_when_down" default:"false"`
	// ReplicaValidityFactor is cluster-replica-validity-factor.
	ReplicaValidityFactor int `yaml:"replica_validity_factor" default:"10"`
	// TLS is tls-cluster: the cluster bus and MIGRATE use mutual TLS with
	// server.tls_cert_file, tls_key_file and tls_ca_cert_file.
	TLS bool `yaml:"tls"`
}

type PersistenceConfig struct {
	Enabled          bool   `yaml:"enabled" default:"false"`
	AOF              bool   `yaml:"aof" default:"true"`
	AOFSync          string `yaml:"aof_sync" default:"everysec"`
	SnapshotInterval string `yaml:"snapshot_interval" default:"5m"`
	DataDir          string `yaml:"data_dir" default:"/var/lib/cachestorm"`
	// AOFRewritePercentage and AOFRewriteMinSize trigger an automatic AOF
	// rewrite, as Redis' auto-aof-rewrite-percentage/min-size do: once the
	// file is at least AOFRewriteMinSize and has grown by AOFRewritePercentage
	// since the last rewrite. A percentage of 0 disables automatic rewrites.
	AOFRewritePercentage int    `yaml:"aof_rewrite_percentage" default:"100"`
	AOFRewriteMinSize    string `yaml:"aof_rewrite_min_size" default:"64mb"`
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
	// BacklogSize is the size in bytes of the replication backlog a
	// reconnecting replica can resume from (repl-backlog-size).
	BacklogSize int `yaml:"backlog_size" default:"1048576"`
	// ReplicaBufferLimit is the most bytes queued for one replica before it
	// is disconnected (client-output-buffer-limit replica).
	ReplicaBufferLimit int `yaml:"replica_buffer_limit" default:"67108864"`
	// TLS makes a replica connect to its master over TLS (tls-replication),
	// presenting server.tls_cert_file and verifying the master against
	// server.tls_ca_cert_file, or the system roots without one.
	TLS bool `yaml:"tls"`
}

// IsReplica reports whether the configuration starts the server as a
// replica.
func (c *ReplicationConfig) IsReplica() bool {
	return c.Role == "replica" || c.Role == "slave"
}

type PluginsConfig struct {
	Metrics MetricsPluginConfig `yaml:"metrics"`
	SlowLog SlowLogConfig       `yaml:"slowlog"`
}

type MetricsPluginConfig struct {
	Enabled bool   `yaml:"enabled" default:"true"`
	Port    int    `yaml:"port" default:"9090"`
	Path    string `yaml:"path" default:"/metrics"`
}

type SlowLogConfig struct {
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
		if err := decodeStrict(data, cfg); err != nil {
			return nil, fmt.Errorf("config %s: %w", path, err)
		}
		if abs, err := filepath.Abs(path); err == nil {
			cfg.SourceFile = abs
		} else {
			cfg.SourceFile = path
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
	// Secrets and the data directory come from the environment in container
	// deployments, so they need not be baked into the image's config file.
	if v := os.Getenv("CACHESTORM_REQUIREPASS"); v != "" {
		c.Server.RequirePass = v
	}
	if v := os.Getenv("CACHESTORM_HTTP_PASSWORD"); v != "" {
		c.HTTP.Password = v
	}
	if v := os.Getenv("CACHESTORM_DATA_DIR"); v != "" {
		c.Persistence.DataDir = v
	}
	if v := os.Getenv("CACHESTORM_PROTECTED_MODE"); v != "" {
		if on, err := strconv.ParseBool(v); err == nil {
			c.Server.ProtectedMode = on
		}
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

// decodeStrict decodes YAML into cfg and refuses keys the schema does not
// know, so a misspelt or retired setting is reported instead of silently
// leaving the default in force.
func decodeStrict(data []byte, cfg *Config) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
