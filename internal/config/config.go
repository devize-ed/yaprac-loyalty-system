package config

import (
	"flag"
	"fmt"
	db "loyaltySys/internal/db/config"
	accrual "loyaltySys/internal/service/accrual/config"
	server "loyaltySys/internal/service/server/config"

	"github.com/caarlos0/env"
)

type Config struct {
	ServerConfig  server.ServerConfig
	AccrualConfig accrual.AccrualConfig
	DBConfig      db.DBConfig
	LogLevel      string `env:"LOG_LEVEL"` // Log level
}

// GetConfig applies the following priority: CLI flags > ENV > default
func GetConfig() (*Config, error) {
	// default config
	cfg := &Config{
		ServerConfig: server.ServerConfig{
			Host: "localhost:8080",
		},
		AccrualConfig: accrual.AccrualConfig{
			AccrualAddr: "http://localhost:8081",
			Timeout:     10,
		},
		DBConfig: db.DBConfig{
			DSN: "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable",
		},
		LogLevel: "debug",
	}

	// parse config from environment variables
	if err := env.Parse(&cfg.ServerConfig); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if err := env.Parse(&cfg.DBConfig); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if err := env.Parse(&cfg.AccrualConfig); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// CLI flags override ENV/default (only if explicitly set)
	flag.StringVar(&cfg.ServerConfig.Host, "a", cfg.ServerConfig.Host, "server address")
	flag.StringVar(&cfg.DBConfig.DSN, "d", cfg.DBConfig.DSN, "database URI")
	flag.StringVar(&cfg.AccrualConfig.AccrualAddr, "r", cfg.AccrualConfig.AccrualAddr, "accrual system address")
	flag.StringVar(&cfg.LogLevel, "l", cfg.LogLevel, "log level")
	flag.IntVar(&cfg.AccrualConfig.Timeout, "t", cfg.AccrualConfig.Timeout, "accrual timeout in seconds")
	flag.Parse()

	return cfg, nil
}
