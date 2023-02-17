package common

func DefaultConfig() Config {
	return Config{
		MaxCollectConnections: 1500,
		MaxConnections:        1500,
	}
}
