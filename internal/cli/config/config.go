package config

func NewConfig() *Config {
	return &Config{
		ServerAddr: "localhost:50051",
	}
}

type Config struct {
	ServerAddr string
}
