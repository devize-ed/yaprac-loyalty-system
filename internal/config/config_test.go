package config

import (
	"flag"
	"os"
	"testing"

	db "loyaltySys/internal/db/config"
	accrual "loyaltySys/internal/service/accrual/config"
	server "loyaltySys/internal/service/server/config"

	"github.com/stretchr/testify/assert"
)

// saveOriginalState saves the original state to restore after tests
func saveOriginalState() ([]string, *flag.FlagSet) {
	originalArgs := make([]string, len(os.Args))
	copy(originalArgs, os.Args)
	originalFlags := flag.CommandLine
	return originalArgs, originalFlags
}

// restoreOriginalState restores the original state
func restoreOriginalState(originalArgs []string, originalFlags *flag.FlagSet) {
	os.Args = originalArgs
	flag.CommandLine = originalFlags
}

func TestGetConfig_PriorityFlagsOverEnv(t *testing.T) {
	originalArgs, originalFlags := saveOriginalState()
	defer restoreOriginalState(originalArgs, originalFlags)

	tests := []struct {
		name     string
		envVars  map[string]string
		args     []string
		expected Config
	}{
		{
			name: "Flags override ENV",
			envVars: map[string]string{
				"RUN_ADDRESS":            "envhost:9000",
				"DATABASE_URI":           "env_dsn",
				"ACCRUAL_SYSTEM_ADDRESS": "http://env-accrual:8081",
				"ACCRUAL_TIMEOUT":        "15",
				"LOG_LEVEL":              "warning",
			},
			args: []string{"-a=:9999", "-d=flag_dsn", "-r=http://flag-accrual:8082", "-l=info", "-t=20"},
			expected: Config{
				ServerConfig: server.ServerConfig{
					Host: ":9999",
				},
				DBConfig: db.DBConfig{
					DSN: "flag_dsn",
				},
				AccrualConfig: accrual.AccrualConfig{
					AccrualAddr: "http://flag-accrual:8082",
					Timeout:     20,
				},
				LogLevel: "info",
			},
		},
		{
			name: "Only ENV (no flags)",
			envVars: map[string]string{
				"RUN_ADDRESS":            "envhost:7777",
				"DATABASE_URI":           "env_dsn_only",
				"ACCRUAL_SYSTEM_ADDRESS": "http://env-accrual-only:8081",
				"ACCRUAL_TIMEOUT":        "25",
				"LOG_LEVEL":              "error",
			},
			args: []string{},
			expected: Config{
				ServerConfig: server.ServerConfig{
					Host: "envhost:7777",
				},
				DBConfig: db.DBConfig{
					DSN: "env_dsn_only",
				},
				AccrualConfig: accrual.AccrualConfig{
					AccrualAddr: "http://env-accrual-only:8081",
					Timeout:     25,
				},
				LogLevel: "error",
			},
		},
		{
			name:    "Defaults (no ENV, no flags)",
			envVars: map[string]string{},
			args:    []string{},
			expected: Config{
				ServerConfig: server.ServerConfig{
					Host: "localhost:8080",
				},
				DBConfig: db.DBConfig{
					DSN: "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable",
				},
				AccrualConfig: accrual.AccrualConfig{
					AccrualAddr: "http://localhost:8081",
					Timeout:     10,
				},
				LogLevel: "debug",
			},
		},
		{
			name: "Partial flags override",
			envVars: map[string]string{
				"RUN_ADDRESS":            "envhost:5555",
				"DATABASE_URI":           "env_dsn_partial",
				"ACCRUAL_SYSTEM_ADDRESS": "http://env-accrual-partial:8081",
				"ACCRUAL_TIMEOUT":        "30",
				"LOG_LEVEL":              "warn",
			},
			args: []string{"-a=:6666", "-l=debug"}, // Only override host and log level
			expected: Config{
				ServerConfig: server.ServerConfig{
					Host: ":6666",
				},
				DBConfig: db.DBConfig{
					DSN: "env_dsn_partial",
				},
				AccrualConfig: accrual.AccrualConfig{
					AccrualAddr: "http://env-accrual-partial:8081",
					Timeout:     30,
				},
				LogLevel: "debug",
			},
		},
	}

	envKeys := []string{"RUN_ADDRESS", "DATABASE_URI", "ACCRUAL_SYSTEM_ADDRESS", "ACCRUAL_TIMEOUT", "LOG_LEVEL"}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new flag set for this test
			flagSet := flag.NewFlagSet(tc.name, flag.ContinueOnError)
			flag.CommandLine = flagSet

			// Clear all environment variables first
			for _, k := range envKeys {
				assert.NoError(t, os.Unsetenv(k))
			}

			// Set environment variables for this test case
			for k, v := range tc.envVars {
				assert.NoError(t, os.Setenv(k, v))
			}

			// Set command line arguments
			os.Args = append([]string{"cmd"}, tc.args...)

			// Get configuration
			cfg, err := GetConfig()
			assert.NoError(t, err)

			// Assert all expected values
			assert.Equal(t, tc.expected.ServerConfig.Host, cfg.ServerConfig.Host, "Server host mismatch")
			assert.Equal(t, tc.expected.DBConfig.DSN, cfg.DBConfig.DSN, "Database DSN mismatch")
			assert.Equal(t, tc.expected.AccrualConfig.AccrualAddr, cfg.AccrualConfig.AccrualAddr, "Accrual address mismatch")
			assert.Equal(t, tc.expected.AccrualConfig.Timeout, cfg.AccrualConfig.Timeout, "Accrual timeout mismatch")
			assert.Equal(t, tc.expected.LogLevel, cfg.LogLevel, "Log level mismatch")
		})
	}
}

func TestGetConfig_EnvironmentVariableParsing(t *testing.T) {
	originalArgs, originalFlags := saveOriginalState()
	defer restoreOriginalState(originalArgs, originalFlags)

	tests := []struct {
		name    string
		envVars map[string]string
		check   func(*testing.T, *Config)
	}{
		{
			name:    "Empty environment variables should use defaults",
			envVars: map[string]string{},
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "localhost:8080", cfg.ServerConfig.Host)
				assert.Equal(t, "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable", cfg.DBConfig.DSN)
				assert.Equal(t, "http://localhost:8081", cfg.AccrualConfig.AccrualAddr)
				assert.Equal(t, 10, cfg.AccrualConfig.Timeout)
				assert.Equal(t, "debug", cfg.LogLevel)
			},
		},
		{
			name: "Custom environment variables should override defaults",
			envVars: map[string]string{
				"RUN_ADDRESS":            "custom-host:9090",
				"DATABASE_URI":           "custom-dsn-string",
				"ACCRUAL_SYSTEM_ADDRESS": "http://custom-accrual:9091",
				"ACCRUAL_TIMEOUT":        "25",
				"LOG_LEVEL":              "info",
			},
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "custom-host:9090", cfg.ServerConfig.Host)
				assert.Equal(t, "custom-dsn-string", cfg.DBConfig.DSN)
				assert.Equal(t, "http://custom-accrual:9091", cfg.AccrualConfig.AccrualAddr)
				assert.Equal(t, 25, cfg.AccrualConfig.Timeout)
				assert.Equal(t, "info", cfg.LogLevel)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new flag set for this test
			flagSet := flag.NewFlagSet(tc.name, flag.ContinueOnError)
			flag.CommandLine = flagSet

			// Clear and set environment variables
			envKeys := []string{"RUN_ADDRESS", "DATABASE_URI", "ACCRUAL_SYSTEM_ADDRESS", "ACCRUAL_TIMEOUT", "LOG_LEVEL"}
			for _, k := range envKeys {
				assert.NoError(t, os.Unsetenv(k))
			}
			for k, v := range tc.envVars {
				assert.NoError(t, os.Setenv(k, v))
			}

			// Set empty command line args
			os.Args = []string{"cmd"}

			// Get configuration
			cfg, err := GetConfig()
			assert.NoError(t, err)

			// Run custom checks
			tc.check(t, cfg)
		})
	}
}

func TestGetConfig_FlagParsing(t *testing.T) {
	originalArgs, originalFlags := saveOriginalState()
	defer restoreOriginalState(originalArgs, originalFlags)

	tests := []struct {
		name     string
		args     []string
		expected Config
	}{
		{
			name: "All flags set",
			args: []string{"-a=:7777", "-d=flag-dsn", "-r=http://flag-accrual:7778", "-l=error", "-t=50"},
			expected: Config{
				ServerConfig: server.ServerConfig{
					Host: ":7777",
				},
				DBConfig: db.DBConfig{
					DSN: "flag-dsn",
				},
				AccrualConfig: accrual.AccrualConfig{
					AccrualAddr: "http://flag-accrual:7778",
					Timeout:     50,
				},
				LogLevel: "error",
			},
		},
		{
			name: "Partial flags set",
			args: []string{"-a=:8888", "-l=warn"},
			expected: Config{
				ServerConfig: server.ServerConfig{
					Host: ":8888",
				},
				DBConfig: db.DBConfig{
					DSN: "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable",
				},
				AccrualConfig: accrual.AccrualConfig{
					AccrualAddr: "http://localhost:8081",
					Timeout:     10,
				},
				LogLevel: "warn",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new flag set for this test
			flagSet := flag.NewFlagSet(tc.name, flag.ContinueOnError)
			flag.CommandLine = flagSet

			// Clear environment variables
			envKeys := []string{"RUN_ADDRESS", "DATABASE_URI", "ACCRUAL_SYSTEM_ADDRESS", "ACCRUAL_TIMEOUT", "LOG_LEVEL"}
			for _, k := range envKeys {
				assert.NoError(t, os.Unsetenv(k))
			}

			// Set command line arguments
			os.Args = append([]string{"cmd"}, tc.args...)

			// Get configuration
			cfg, err := GetConfig()
			assert.NoError(t, err)

			// Assert expected values
			assert.Equal(t, tc.expected.ServerConfig.Host, cfg.ServerConfig.Host, "Server host mismatch")
			assert.Equal(t, tc.expected.DBConfig.DSN, cfg.DBConfig.DSN, "Database DSN mismatch")
			assert.Equal(t, tc.expected.AccrualConfig.AccrualAddr, cfg.AccrualConfig.AccrualAddr, "Accrual address mismatch")
			assert.Equal(t, tc.expected.AccrualConfig.Timeout, cfg.AccrualConfig.Timeout, "Accrual timeout mismatch")
			assert.Equal(t, tc.expected.LogLevel, cfg.LogLevel, "Log level mismatch")
		})
	}
}
