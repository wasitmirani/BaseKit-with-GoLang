package config

import "time"

// AppConfig holds application-level configuration
type AppConfig struct {
	Name        string
	Version     string
	Environment string
	Debug       bool
	URL         string
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         string
	Host         string
	Mode         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Channel     string // stdout, file, stack
	Level       string // debug, info, warn, error
	LogPath     string // Directory path for log files
	LogFileName string // Base name for log files
	MaxSize     int    // Maximum size in MB before rotation
	MaxBackups  int    // Maximum number of old log files to retain
	MaxAge      int    // Maximum number of days to retain old log files
	Compress    bool   // Whether to compress rotated log files
	DailyRotate bool   // Enable daily rotation
}

// CORSConfig holds Cross-Origin Resource Sharing settings
type CORSConfig struct {
	AllowedOrigins   string
	AllowedMethods   string
	AllowedHeaders   string
	AllowCredentials bool
}

// RateLimitConfig holds API rate limiting settings
type RateLimitConfig struct {
	Enabled     bool
	MaxRequests int
	Window      time.Duration
}

// APIConfig holds API route settings
type APIConfig struct {
	Prefix    string
	RateLimit int
}

func loadAppConfig() AppConfig {
	return AppConfig{
		Name:        getString("APP_NAME", "BaseKit Service"),
		Version:     getString("APP_VERSION", "1.0.0"),
		Environment: getString("APP_ENV", "development"),
		Debug:       getBool("APP_DEBUG", true),
		URL:         getString("APP_URL", "http://localhost:3000"),
	}
}

func loadServerConfig() ServerConfig {
	return ServerConfig{
		Port:         getString("SERVER_PORT", "8080"),
		Host:         getString("SERVER_HOST", "0.0.0.0"),
		Mode:         getString("GIN_MODE", "debug"),
		ReadTimeout:  getDuration("SERVER_READ_TIMEOUT", 15*time.Second),
		WriteTimeout: getDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
		IdleTimeout:  getDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
	}
}

func loadLoggingConfig() LoggingConfig {
	return LoggingConfig{
		Channel:     getString("LOG_CHANNEL", "stdout"),
		Level:       getString("LOG_LEVEL", "debug"),
		LogPath:     getString("LOG_FILE_PATH", "./storage/logs"),
		LogFileName: getString("LOG_FILE_NAME", "app"),
		MaxSize:     getInt("LOG_MAX_SIZE", 10),
		MaxBackups:  getInt("LOG_MAX_BACKUPS", 5),
		MaxAge:      getInt("LOG_MAX_AGE", 28),
		Compress:    getBool("LOG_COMPRESS", true),
		DailyRotate: getBool("LOG_DAILY_ROTATE", true),
	}
}

func loadCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   getString("CORS_ALLOWED_ORIGINS", "*"),
		AllowedMethods:   getString("CORS_ALLOWED_METHODS", "GET,POST,PUT,DELETE,OPTIONS"),
		AllowedHeaders:   getString("CORS_ALLOWED_HEADERS", "*"),
		AllowCredentials: getBool("CORS_ALLOW_CREDENTIALS", true),
	}
}

func loadRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:     getBool("RATE_LIMIT_ENABLED", true),
		MaxRequests: getInt("RATE_LIMIT_MAX_REQUESTS", 60),
		Window:      getDuration("RATE_LIMIT_WINDOW", time.Minute),
	}
}

func loadAPIConfig() APIConfig {
	return APIConfig{
		Prefix:    getString("API_PREFIX", "/api/v1"),
		RateLimit: getInt("API_RATE_LIMIT", 60),
	}
}
