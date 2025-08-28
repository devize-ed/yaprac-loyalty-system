package config

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfig_PriorityFlagsOverEnv(t *testing.T) {
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
				"LOG_LEVEL":              "warning",
			},
			args: []string{"-a=:9999", "-d=flag_dsn", "-r=http://flag-accrual:8082", "-l=info"},
			expected: Config{
				Host:        ":9999",
				DSN:         "flag_dsn",
				AccrualAddr: "http://flag-accrual:8082",
				LogLevel:    "info",
			},
		},
		{
			name: "Only ENV (no flags)",
			envVars: map[string]string{
				"RUN_ADDRESS":            "envhost:7777",
				"DATABASE_URI":           "env_dsn_only",
				"ACCRUAL_SYSTEM_ADDRESS": "http://env-accrual-only:8081",
				"LOG_LEVEL":              "error",
			},
			args: []string{},
			expected: Config{
				Host:        "envhost:7777",
				DSN:         "env_dsn_only",
				AccrualAddr: "http://env-accrual-only:8081",
				LogLevel:    "error",
			},
		},
		{
			name:    "Defaults (no ENV, no flags)",
			envVars: map[string]string{},
			args:    []string{},
			expected: Config{
				Host:        "localhost:8080",
				DSN:         "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable",
				AccrualAddr: "http://localhost:8081",
				LogLevel:    "debug",
			},
		},
	}

	envKeys := []string{"RUN_ADDRESS", "DATABASE_URI", "ACCRUAL_SYSTEM_ADDRESS", "LOG_LEVEL"}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet(tc.name, flag.ContinueOnError)

			for _, k := range envKeys {
				t.Setenv(k, "")
			}
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			os.Args = append([]string{"cmd"}, tc.args...)

			cfg, err := GetConfig()
			assert.NoError(t, err)

			assert.Equal(t, tc.expected.Host, cfg.Host)
			assert.Equal(t, tc.expected.DSN, cfg.DSN)
			assert.Equal(t, tc.expected.AccrualAddr, cfg.AccrualAddr)
			assert.Equal(t, tc.expected.LogLevel, cfg.LogLevel)
		})
	}
}
