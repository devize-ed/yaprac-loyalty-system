package config

// Accrual service configuration. Timeout is specified in seconds.
type AccrualConfig struct {
	AccrualAddr string `env:"ACCRUAL_SYSTEM_ADDRESS"` // Accrual system address
	Timeout     int    `env:"ACCRUAL_TIMEOUT"`        // Timeout in seconds for accrual requests
}
