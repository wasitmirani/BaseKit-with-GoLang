package config

import (
	"log"
	"os"
	"time"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

// Config holds all application configuration
type Config struct {
	App       AppConfig
	Server    ServerConfig
	Database  DatabaseConfig
	JWT       JWTConfig
	Logging   LoggingConfig
	CORS      CORSConfig
	RateLimit RateLimitConfig
	API       APIConfig
	Cache     CacheConfig
	Redis     RedisConfig
	Session   SessionConfig
	Queue     QueueConfig
	Services  ServicesConfig
}

// LoadConfig loads configuration from environment variables and config files
func LoadConfig() (*Config, error) {
	// Load .env file if it exists (for local development)
	// This must be done before viper.AutomaticEnv() to ensure .env values are available
	if err := gotenv.Load(); err != nil {
		// .env file is optional, so we only log a warning if it doesn't exist
		// This allows the app to work with system environment variables only
		if !os.IsNotExist(err) {
			log.Printf("Warning: Failed to load .env file: %v", err)
		}
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("../config")

	// Enable environment variables (reads from both .env file and system env)
	// System environment variables take precedence over .env file values
	viper.AutomaticEnv()

	// Set defaults
	setDefaults()

	// Try to read config file (optional)
	_ = viper.ReadInConfig()

	cfg := &Config{
		App:       loadAppConfig(),
		Server:    loadServerConfig(),
		Database:  loadDatabaseConfig(),
		JWT:       loadJWTConfig(),
		Logging:   loadLoggingConfig(),
		CORS:      loadCORSConfig(),
		RateLimit: loadRateLimitConfig(),
		API:       loadAPIConfig(),
		Cache:     loadCacheConfig(),
		Redis:     loadRedisConfig(),
		Session:   loadSessionConfig(),
		Queue:     loadQueueConfig(),
		Services:  loadServicesConfig(),
	}

	return cfg, nil
}

func setDefaults() {
	viper.SetDefault("SERVER_PORT", "8080")
	viper.SetDefault("SERVER_HOST", "0.0.0.0")
	viper.SetDefault("GIN_MODE", "debug")
	viper.SetDefault("DB_DRIVER", "postgresql")
	viper.SetDefault("DB_PORT", "5432")
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("CACHE_DRIVER", "memory")
	viper.SetDefault("SESSION_DRIVER", "memory")
	viper.SetDefault("QUEUE_CONNECTION", "sync")
	viper.SetDefault("FILESYSTEM_DISK", "local")
}

func getString(key, defaultValue string) string {
	if value := viper.GetString(key); value != "" {
		return value
	}
	return defaultValue
}

func getInt(key string, defaultValue int) int {
	if value := viper.GetInt(key); value != 0 {
		return value
	}
	return defaultValue
}

func getBool(key string, defaultValue bool) bool {
	if viper.IsSet(key) {
		return viper.GetBool(key)
	}
	return defaultValue
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value := viper.GetDuration(key); value != 0 {
		return value
	}
	return defaultValue
}
