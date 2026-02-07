package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	ClickHouse ClickHouseConfig `yaml:"clickhouse"`
	PostgreSQL PostgreSQLConfig `yaml:"postgresql"`
}

type ServerConfig struct {
	GRPCPort int    `yaml:"grpc_port"`
	HTTPPort int    `yaml:"http_port"`
	APIKey   string `yaml:"api_key"`
	AuthMode string `yaml:"auth_mode"` // "static" (default) | "multi_tenant"
	AdminKey string `yaml:"admin_key"` // static admin key for admin API
}

type ClickHouseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type PostgreSQLConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			GRPCPort: 4317,
			HTTPPort: 8080,
			AuthMode: "static",
		},
		ClickHouse: ClickHouseConfig{
			Host:     "localhost",
			Port:     9000,
			Database: "axonize",
			User:     "default",
		},
		PostgreSQL: PostgreSQLConfig{
			Host:     "localhost",
			Port:     5432,
			Database: "axonize",
			User:     "axonize",
			Password: "axonize",
		},
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// ApplyEnv overrides config values with environment variables when set.
func (c *Config) ApplyEnv() {
	if v := os.Getenv("AXONIZE_GRPC_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Server.GRPCPort = p
		}
	}
	if v := os.Getenv("AXONIZE_HTTP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Server.HTTPPort = p
		}
	}

	if v := os.Getenv("CLICKHOUSE_HOST"); v != "" {
		c.ClickHouse.Host = v
	}
	if v := os.Getenv("CLICKHOUSE_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.ClickHouse.Port = p
		}
	}
	if v := os.Getenv("CLICKHOUSE_DATABASE"); v != "" {
		c.ClickHouse.Database = v
	}
	if v := os.Getenv("CLICKHOUSE_USER"); v != "" {
		c.ClickHouse.User = v
	}
	if v := os.Getenv("CLICKHOUSE_PASSWORD"); v != "" {
		c.ClickHouse.Password = v
	}

	if v := os.Getenv("POSTGRES_HOST"); v != "" {
		c.PostgreSQL.Host = v
	}
	if v := os.Getenv("POSTGRES_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.PostgreSQL.Port = p
		}
	}
	if v := os.Getenv("POSTGRES_DATABASE"); v != "" {
		c.PostgreSQL.Database = v
	}
	if v := os.Getenv("POSTGRES_USER"); v != "" {
		c.PostgreSQL.User = v
	}
	if v := os.Getenv("POSTGRES_PASSWORD"); v != "" {
		c.PostgreSQL.Password = v
	}

	if v := os.Getenv("AXONIZE_API_KEY"); v != "" {
		c.Server.APIKey = v
	}
	if v := os.Getenv("AXONIZE_AUTH_MODE"); v != "" {
		c.Server.AuthMode = v
	}
	if v := os.Getenv("AXONIZE_ADMIN_KEY"); v != "" {
		c.Server.AdminKey = v
	}
}
