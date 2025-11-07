package config

type ServerConfig struct {
	Host string `env:"RUN_ADDRESS"` // Server address
}
