package config

import "time"

type AccrualConfig struct {
	AccrualAddr string        `env:"ACCRUAL_SYSTEM_ADDRESS"`       // Accrual system address
	Timeout     time.Duration `env:"ACCRUAL_TIMEOUT" default:"10"` // Timeout for accrual request
}
