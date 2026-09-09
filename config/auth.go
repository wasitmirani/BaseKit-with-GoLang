package config

import "time"

// JWTConfig holds JWT authentication configuration
type JWTConfig struct {
	Secret     string
	Expiration time.Duration
	Issuer     string
}

func loadJWTConfig() JWTConfig {
	return JWTConfig{
		Secret:     getString("JWT_SECRET", "your-secret-key-change-in-production"),
		Expiration: getDuration("JWT_EXPIRATION", 24*time.Hour),
		Issuer:     getString("JWT_ISSUER", "backoffice-service"),
	}
}
