package config

import "time"

// CacheConfig holds cache configuration
type CacheConfig struct {
	Driver string // memory, redis, file
	Prefix string
	TTL    time.Duration
}

// RedisConfig holds Redis connection settings
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
	PoolSize int
}

func loadCacheConfig() CacheConfig {
	ttlSeconds := getInt("CACHE_TTL", 3600)
	return CacheConfig{
		Driver: getString("CACHE_DRIVER", "memory"),
		Prefix: getString("CACHE_PREFIX", "backoffice_cache"),
		TTL:    time.Duration(ttlSeconds) * time.Second,
	}
}

func loadRedisConfig() RedisConfig {
	return RedisConfig{
		Host:     getString("REDIS_HOST", "127.0.0.1"),
		Port:     getString("REDIS_PORT", "6379"),
		Password: getString("REDIS_PASSWORD", ""),
		DB:       getInt("REDIS_DB", 0),
		PoolSize: getInt("REDIS_POOL_SIZE", 10),
	}
}
