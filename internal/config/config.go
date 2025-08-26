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

func GetConfig() (*Config, error) {
	cfg := &Config{}

	// Parse environment variables
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	// Override environment variables with command line arguments if provided
	flag.StringVar(&cfg.Host, "a", "localhost:8080", "server address")
	flag.StringVar(&cfg.DSN, "d", "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable", "database URI")
	flag.StringVar(&cfg.AccrualAddr, "r", "http://localhost:8081", "accrual system address")
	flag.StringVar(&cfg.LogLevel, "l", "debug", "log level")
	flag.Parse()
	return cfg, nil
}
