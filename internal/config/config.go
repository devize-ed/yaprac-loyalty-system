package config

import (
	"flag"
	"fmt"

	"github.com/caarlos0/env"
)

type Config struct {
	Host        string `env:"RUN_ADDRESS"`            // Server address
	DSN         string `env:"DATABASE_URI"`           // Database URI
	AccrualAddr string `env:"ACCRUAL_SYSTEM_ADDRESS"` // Accrual system address
	LogLevel    string `env:"LOG_LEVEL"`              // Log level
}

// GetConfig applies the following priority: CLI flags > ENV > default
func GetConfig() (*Config, error) {
	// default config
	cfg := &Config{
		Host:        "localhost:8080",
		DSN:         "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable",
		AccrualAddr: "http://localhost:8081",
		LogLevel:    "debug",
	}

	// parse config from environment variables
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// CLI flags override ENV/default (only if explicitly set)
	flag.StringVar(&cfg.Host, "a", cfg.Host, "server address")
	flag.StringVar(&cfg.DSN, "d", cfg.DSN, "database URI")
	flag.StringVar(&cfg.AccrualAddr, "r", cfg.AccrualAddr, "accrual system address")
	flag.StringVar(&cfg.LogLevel, "l", cfg.LogLevel, "log level")
	flag.Parse()

	return cfg, nil
}
