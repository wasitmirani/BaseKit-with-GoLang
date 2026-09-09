package config

// SessionConfig holds session configuration
type SessionConfig struct {
	Driver   string // memory, redis, cookie
	Lifetime int    // minutes
	Encrypt  bool
	Path     string
	Domain   string
}

func loadSessionConfig() SessionConfig {
	return SessionConfig{
		Driver:   getString("SESSION_DRIVER", "memory"),
		Lifetime: getInt("SESSION_LIFETIME", 120),
		Encrypt:  getBool("SESSION_ENCRYPT", false),
		Path:     getString("SESSION_PATH", "/"),
		Domain:   getString("SESSION_DOMAIN", ""),
	}
}
