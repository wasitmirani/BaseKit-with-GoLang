package config

// ServicesConfig holds third-party and infrastructure service settings
type ServicesConfig struct {
	Mail     MailConfig
	Storage  StorageConfig
	Sentry   SentryConfig
	NewRelic NewRelicConfig
}

// MailConfig holds email/SMTP configuration
type MailConfig struct {
	Mailer      string
	Host        string
	Port        string
	Username    string
	Password    string
	Encryption  string
	FromAddress string
	FromName    string
}

// StorageConfig holds file storage configuration
type StorageConfig struct {
	Disk  string // local, s3, azure
	Path  string
	AWS   AWSConfig
	Azure AzureConfig
}

// AWSConfig holds AWS S3 settings
type AWSConfig struct {
	AccessKeyID     string
	SecretAccessKey string
	DefaultRegion   string
	Bucket          string
	Endpoint        string
}

// AzureConfig holds Azure Blob Storage settings
type AzureConfig struct {
	StorageAccount   string
	StorageKey       string
	StorageContainer string
}

// SentryConfig holds Sentry error tracking settings
type SentryConfig struct {
	DSN         string
	Environment string
}

// NewRelicConfig holds New Relic APM settings
type NewRelicConfig struct {
	LicenseKey string
	AppName    string
}

func loadServicesConfig() ServicesConfig {
	appName := getString("APP_NAME", "BaseKit Service")

	return ServicesConfig{
		Mail: MailConfig{
			Mailer:      getString("MAIL_MAILER", "smtp"),
			Host:        getString("MAIL_HOST", "smtp.mailtrap.io"),
			Port:        getString("MAIL_PORT", "2525"),
			Username:    getString("MAIL_USERNAME", ""),
			Password:    getString("MAIL_PASSWORD", ""),
			Encryption:  getString("MAIL_ENCRYPTION", "tls"),
			FromAddress: getString("MAIL_FROM_ADDRESS", "noreply@example.com"),
			FromName:    getString("MAIL_FROM_NAME", appName),
		},
		Storage: StorageConfig{
			Disk: getString("FILESYSTEM_DISK", "local"),
			Path: getString("STORAGE_PATH", "./storage/app"),
			AWS: AWSConfig{
				AccessKeyID:     getString("AWS_ACCESS_KEY_ID", ""),
				SecretAccessKey: getString("AWS_SECRET_ACCESS_KEY", ""),
				DefaultRegion:   getString("AWS_DEFAULT_REGION", "us-east-1"),
				Bucket:          getString("AWS_BUCKET", ""),
				Endpoint:        getString("AWS_ENDPOINT", ""),
			},
			Azure: AzureConfig{
				StorageAccount:   getString("AZURE_STORAGE_ACCOUNT", ""),
				StorageKey:       getString("AZURE_STORAGE_KEY", ""),
				StorageContainer: getString("AZURE_STORAGE_CONTAINER", ""),
			},
		},
		Sentry: SentryConfig{
			DSN:         getString("SENTRY_DSN", ""),
			Environment: getString("SENTRY_ENVIRONMENT", "production"),
		},
		NewRelic: NewRelicConfig{
			LicenseKey: getString("NEW_RELIC_LICENSE_KEY", ""),
			AppName:    getString("NEW_RELIC_APP_NAME", appName),
		},
	}
}
