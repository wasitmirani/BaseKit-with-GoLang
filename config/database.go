package config

import (
	"time"

	"BaseKit-with-GoLang/internal/pkg/database"
)

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	// Primary database (default)
	Primary DatabaseConnectionConfig `mapstructure:"primary"`

	// Additional databases can be added here
	Secondary DatabaseConnectionConfig `mapstructure:"secondary"`

	// Multiple databases support
	Databases map[string]DatabaseConnectionConfig `mapstructure:"databases"`

	// Startup behavior
	Seed    bool
	Migrate bool
}

// DatabaseConnectionConfig holds configuration for a single database connection
type DatabaseConnectionConfig struct {
	Driver          string        `mapstructure:"driver"` // postgresql, mysql, mongodb, sqlite
	Host            string        `mapstructure:"host"`
	Port            string        `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"` // For PostgreSQL
	Charset         string        `mapstructure:"charset"` // For MySQL
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	UseGorm         bool          `mapstructure:"use_gorm"`
}

func loadDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Primary: DatabaseConnectionConfig{
			Driver:          getString("DB_DRIVER", "postgresql"),
			Host:            getString("DB_HOST", "localhost"),
			Port:            getString("DB_PORT", "5432"),
			User:            getString("DB_USER", "postgres"),
			Password:        getString("DB_PASSWORD", ""),
			DBName:          getString("DB_NAME", "backoffice"),
			SSLMode:         getString("DB_SSLMODE", "disable"),
			Charset:         getString("DB_CHARSET", "utf8mb4"),
			MaxOpenConns:    getInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			ConnMaxIdleTime: getDuration("DB_CONN_MAX_IDLE_TIME", 10*time.Minute),
			UseGorm:         getBool("DB_USE_GORM", true),
		},
		Databases: make(map[string]DatabaseConnectionConfig),
		Seed:      getBool("DB_SEED", false),
		Migrate:   getBool("DB_MIGRATE", false),
	}
}

// GetDatabaseDriverConfig converts DatabaseConnectionConfig to appropriate driver config
func (dbc *DatabaseConnectionConfig) GetDatabaseDriverConfig() (database.DriverType, interface{}, error) {
	driverType := database.DriverType(dbc.Driver)

	switch driverType {
	case database.DriverPostgreSQL:
		return driverType, &database.PostgresConfig{
			Host:            dbc.Host,
			Port:            dbc.Port,
			User:            dbc.User,
			Password:        dbc.Password,
			DBName:          dbc.DBName,
			SSLMode:         dbc.SSLMode,
			MaxOpenConns:    dbc.MaxOpenConns,
			MaxIdleConns:    dbc.MaxIdleConns,
			ConnMaxLifetime: dbc.ConnMaxLifetime,
			ConnMaxIdleTime: dbc.ConnMaxIdleTime,
			UseGorm:         dbc.UseGorm,
		}, nil

	case database.DriverMySQL:
		return driverType, &database.MySQLConfig{
			Host:            dbc.Host,
			Port:            dbc.Port,
			User:            dbc.User,
			Password:        dbc.Password,
			DBName:          dbc.DBName,
			Charset:         dbc.Charset,
			ParseTime:       true,
			Loc:             "Local",
			MaxOpenConns:    dbc.MaxOpenConns,
			MaxIdleConns:    dbc.MaxIdleConns,
			ConnMaxLifetime: dbc.ConnMaxLifetime,
			ConnMaxIdleTime: dbc.ConnMaxIdleTime,
			UseGorm:         dbc.UseGorm,
		}, nil

	default:
		return "", nil, database.ErrUnsupportedDriver
	}
}
