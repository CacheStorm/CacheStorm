package config

import (
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PlaceholderPassword is the password written in the production config
// template.
const PlaceholderPassword = "CHANGE_ME_TO_A_STRONG_PASSWORD"

func Validate(cfg *Config) error {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	// The production template ships this placeholder; running with it would
	// publish a password anyone can read in the repository.
	if cfg.Server.RequirePass == PlaceholderPassword || cfg.HTTP.Password == PlaceholderPassword {
		return fmt.Errorf("replace the placeholder password %q before starting", PlaceholderPassword)
	}

	if cfg.Server.MaxConnections < 0 {
		return fmt.Errorf("max_connections cannot be negative")
	}
	if cfg.Server.Databases < 0 || cfg.Server.Databases > 1<<20 {
		return fmt.Errorf("invalid databases: %d", cfg.Server.Databases)
	}
	if cfg.Server.TCPKeepAlive < 0 {
		return fmt.Errorf("tcp_keepalive cannot be negative")
	}
	if cfg.Server.ReadBufferSize < 0 || cfg.Server.ReadBufferSize > 64<<20 {
		return fmt.Errorf("invalid read_buffer_size: %d", cfg.Server.ReadBufferSize)
	}
	if cfg.Server.WriteBufferSize < 0 || cfg.Server.WriteBufferSize > 64<<20 {
		return fmt.Errorf("invalid write_buffer_size: %d", cfg.Server.WriteBufferSize)
	}
	maxMemory, err := ParseMemorySize(cfg.Memory.MaxMemory)
	if err != nil || maxMemory < 0 {
		return fmt.Errorf("invalid max_memory: %s", cfg.Memory.MaxMemory)
	}

	if cfg.Memory.WarningPct < 0 || cfg.Memory.WarningPct > 100 {
		return fmt.Errorf("warning percentage must be 0-100")
	}

	if cfg.Memory.CriticalPct < 0 || cfg.Memory.CriticalPct > 100 {
		return fmt.Errorf("critical percentage must be 0-100")
	}

	if cfg.Memory.WarningPct >= cfg.Memory.CriticalPct {
		return fmt.Errorf("warning percentage must be less than critical percentage")
	}

	validPolicies := map[string]bool{
		"noeviction":      true,
		"allkeys-lru":     true,
		"allkeys-lfu":     true,
		"volatile-lru":    true,
		"allkeys-random":  true,
		"volatile-lfu":    true,
		"volatile-random": true,
		"volatile-ttl":    true,
	}
	if !validPolicies[strings.ToLower(cfg.Memory.EvictionPolicy)] {
		return fmt.Errorf("invalid eviction policy: %s", cfg.Memory.EvictionPolicy)
	}
	cfg.Memory.EvictionPolicy = strings.ToLower(cfg.Memory.EvictionPolicy)

	if cfg.Persistence.Enabled && cfg.Persistence.SnapshotInterval != "" {
		interval, err := time.ParseDuration(cfg.Persistence.SnapshotInterval)
		if err != nil || interval < 0 {
			return fmt.Errorf("invalid persistence.snapshot_interval: %q", cfg.Persistence.SnapshotInterval)
		}
	}

	if cfg.Persistence.AOFRewritePercentage < 0 {
		return fmt.Errorf("invalid persistence.aof_rewrite_percentage: %d", cfg.Persistence.AOFRewritePercentage)
	}
	if size, err := ParseMemorySize(cfg.Persistence.AOFRewriteMinSize); err != nil || size < 0 {
		return fmt.Errorf("invalid persistence.aof_rewrite_min_size: %q", cfg.Persistence.AOFRewriteMinSize)
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[strings.ToLower(cfg.Logging.Level)] {
		return fmt.Errorf("invalid log level: %s", cfg.Logging.Level)
	}

	switch cfg.Replication.Role {
	case "", "master":
	case "replica", "slave":
		if cfg.Replication.MasterHost == "" || cfg.Replication.MasterPort < 1 || cfg.Replication.MasterPort > 65535 {
			return fmt.Errorf("replication.master_host and replication.master_port are required when replication.role is %s", cfg.Replication.Role)
		}
	default:
		return fmt.Errorf("invalid replication role: %s", cfg.Replication.Role)
	}

	// Validate bind address
	if cfg.Server.Bind != "" && cfg.Server.Bind != "0.0.0.0" {
		if net.ParseIP(cfg.Server.Bind) == nil {
			return fmt.Errorf("invalid bind address: %s", cfg.Server.Bind)
		}
	}

	// Validate HTTP port
	if cfg.HTTP.Enabled {
		if cfg.HTTP.Port < 1 || cfg.HTTP.Port > 65535 {
			return fmt.Errorf("invalid HTTP port: %d", cfg.HTTP.Port)
		}
	}

	if cfg.Cluster.Enabled {
		if err := validateCluster(cfg); err != nil {
			return err
		}
	}

	// Validate TLS files exist when specified
	if cfg.Server.TLSCertFile != "" {
		if _, err := os.Stat(cfg.Server.TLSCertFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS cert file not found: %s", cfg.Server.TLSCertFile)
		}
	}
	if cfg.Server.TLSKeyFile != "" {
		if _, err := os.Stat(cfg.Server.TLSKeyFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS key file not found: %s", cfg.Server.TLSKeyFile)
		}
	}

	return nil
}

func ParseMemorySize(s string) (int64, error) {
	if s == "0" || s == "" {
		return 0, nil
	}

	s = strings.TrimSpace(strings.ToLower(s))

	suffixes := []struct {
		suffix string
		mult   int64
	}{
		{"tib", 1024 * 1024 * 1024 * 1024},
		{"gib", 1024 * 1024 * 1024},
		{"mib", 1024 * 1024},
		{"kib", 1024},
		{"tb", 1024 * 1024 * 1024 * 1024},
		{"gb", 1024 * 1024 * 1024},
		{"mb", 1024 * 1024},
		{"kb", 1024},
		{"b", 1},
	}

	for _, sf := range suffixes {
		if strings.HasSuffix(s, sf.suffix) {
			numStr := strings.TrimSuffix(s, sf.suffix)
			num, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid memory size: %s", s)
			}
			if num > math.MaxInt64/sf.mult || num < math.MinInt64/sf.mult {
				return 0, fmt.Errorf("invalid memory size: %s", s)
			}
			return num * sf.mult, nil
		}
	}

	num, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory size: %s", s)
	}
	return num, nil
}

// validateCluster checks the cluster section of a configuration that enables
// cluster mode.
func validateCluster(cfg *Config) error {
	c := cfg.Cluster
	if cfg.Replication.IsReplica() {
		return fmt.Errorf("replication.role %s cannot be combined with cluster mode: replicas are attached with CLUSTER REPLICATE", cfg.Replication.Role)
	}
	if c.ConfigFile == "" || filepath.Base(c.ConfigFile) != c.ConfigFile {
		return fmt.Errorf("invalid cluster config_file %q: it must be a file name inside persistence.data_dir", c.ConfigFile)
	}
	if c.NodeTimeout <= 0 {
		return fmt.Errorf("invalid cluster node_timeout: %d", c.NodeTimeout)
	}
	if c.ReplicaValidityFactor < 0 {
		return fmt.Errorf("invalid cluster replica_validity_factor: %d", c.ReplicaValidityFactor)
	}
	for name, port := range map[string]int{"bus_port": c.BusPort, "announce_port": c.AnnouncePort, "announce_bus_port": c.AnnounceBusPort} {
		if port < 0 || port > 65535 {
			return fmt.Errorf("invalid cluster %s: %d", name, port)
		}
	}
	if c.BusPort == 0 && cfg.Server.Port+10000 > 65535 {
		return fmt.Errorf("port %d is too high for cluster mode: the cluster bus port is the port + 10000; set cluster.bus_port", cfg.Server.Port)
	}
	if c.AnnounceIP != "" && net.ParseIP(c.AnnounceIP) == nil {
		return fmt.Errorf("invalid cluster announce_ip: %s", c.AnnounceIP)
	}
	return nil
}
