package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

func Validate(cfg *Config) error {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if cfg.Server.MaxConnections < 0 {
		return fmt.Errorf("max_connections cannot be negative")
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
		"noeviction":     true,
		"allkeys-lru":    true,
		"allkeys-lfu":    true,
		"volatile-lru":   true,
		"allkeys-random": true,
	}
	if !validPolicies[strings.ToLower(cfg.Memory.EvictionPolicy)] {
		return fmt.Errorf("invalid eviction policy: %s", cfg.Memory.EvictionPolicy)
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

	if cfg.Cluster.Enabled && cfg.Cluster.NodeName == "" {
		return fmt.Errorf("node_name is required when cluster is enabled")
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

	// Validate cluster bind port
	if cfg.Cluster.Enabled && cfg.Cluster.BindPort != 0 {
		if cfg.Cluster.BindPort < 1 || cfg.Cluster.BindPort > 65535 {
			return fmt.Errorf("invalid cluster bind port: %d", cfg.Cluster.BindPort)
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
			return num * sf.mult, nil
		}
	}

	num, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory size: %s", s)
	}
	return num, nil
}
