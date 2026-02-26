package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Load reads all YAML configs from the given directory and returns a Config.
func Load(dir string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:            8080,
			ShutdownTimeout: 30,
		},
		GeoIP: GeoIPConfig{
			DatabasePath: "/data/GeoLite2-Country.mmdb",
			CacheTTLMin:  10,
		},
		Auth: AuthConfig{
			APIKeyHeader: "X-API-Key",
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 100,
			Burst:             200,
		},
		Audit: AuditConfig{
			OutputType: "stdout",
		},
	}

	if err := loadYAML(filepath.Join(dir, "routing-rules.yaml"), &cfg.RoutingRules); err != nil {
		return nil, fmt.Errorf("loading routing-rules.yaml: %w", err)
	}

	if err := loadYAML(filepath.Join(dir, "providers.yaml"), &cfg.Providers); err != nil {
		return nil, fmt.Errorf("loading providers.yaml: %w", err)
	}

	if err := loadYAML(filepath.Join(dir, "compliance-rules.yaml"), &cfg.Compliance); err != nil {
		return nil, fmt.Errorf("loading compliance-rules.yaml: %w", err)
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

// loadYAML unmarshals a YAML file into the given target.
func loadYAML(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	return nil
}

// applyEnvOverrides applies environment variable overrides to the config.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("GATEWAY_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("GEOIP_DB_PATH"); v != "" {
		cfg.GeoIP.DatabasePath = v
	}
}
