package config

const (
	CgroupsBaseDir = "/sys/fs/cgroup"
)

func NewConfig() *Config {
	return &Config{
		ServerAddr: "localhost:50051",
	}
}

type Config struct {
	ServerAddr string
}
