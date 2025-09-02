package config

type DBConfig struct {
	DSN string `env:"DATABASE_URI"` // Database URI
}
