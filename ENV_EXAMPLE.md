# .env.example File Content

Since `.env.example` creation was blocked, here's the content to create manually:

```bash
# ============================================
# Application Configuration
# ============================================
APP_NAME="Backoffice Service"
APP_VERSION="1.0.0"
APP_ENV=local
APP_DEBUG=true
APP_URL=http://localhost:8080

# ============================================
# Server Configuration
# ============================================
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
GIN_MODE=debug

# Server Timeouts (in seconds)
SERVER_READ_TIMEOUT=15s
SERVER_WRITE_TIMEOUT=90s
SERVER_IDLE_TIMEOUT=60s

# ============================================
# Primary Database Configuration
# ============================================
DB_DRIVER=postgresql
DB_HOST=127.0.0.1
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=
DB_NAME=backoffice
DB_SSLMODE=disable

# Database Connection Pool Settings
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME=5m
DB_CONN_MAX_IDLE_TIME=10m

# Use GORM ORM (true/false)
DB_USE_GORM=true

# ============================================
# Secondary Database (Optional)
# ============================================
# DB_SECONDARY_DRIVER=mysql
# DB_SECONDARY_HOST=127.0.0.1
# DB_SECONDARY_PORT=3306
# DB_SECONDARY_USER=root
# DB_SECONDARY_PASSWORD=
# DB_SECONDARY_DBNAME=analytics
# DB_SECONDARY_CHARSET=utf8mb4
# DB_SECONDARY_USE_GORM=true

# ============================================
# JWT Authentication Configuration
# ============================================
JWT_SECRET=your-secret-key-change-in-production-min-32-characters-long
JWT_EXPIRATION=24h
JWT_ISSUER=backoffice-service

# ============================================
# Redis Configuration (Optional)
# ============================================
REDIS_HOST=127.0.0.1
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_POOL_SIZE=10

# ============================================
# Email Configuration (Optional)
# ============================================
MAIL_MAILER=smtp
MAIL_HOST=smtp.mailtrap.io
MAIL_PORT=2525
MAIL_USERNAME=
MAIL_PASSWORD=
MAIL_ENCRYPTION=tls
MAIL_FROM_ADDRESS=noreply@example.com
MAIL_FROM_NAME="${APP_NAME}"

# ============================================
# File Storage Configuration (Optional)
# ============================================
FILESYSTEM_DISK=local
# FILESYSTEM_DISK=s3

# Local Storage
STORAGE_PATH=./storage/app

# AWS S3 Configuration
AWS_ACCESS_KEY_ID=
AWS_SECRET_ACCESS_KEY=
AWS_DEFAULT_REGION=us-east-1
AWS_BUCKET=
AWS_ENDPOINT=

# Azure Blob Storage Configuration
AZURE_STORAGE_ACCOUNT=
AZURE_STORAGE_KEY=
AZURE_STORAGE_CONTAINER=

# ============================================
# Message Queue Configuration (Optional)
# ============================================
QUEUE_CONNECTION=sync
# QUEUE_CONNECTION=redis
# QUEUE_CONNECTION=database

# Redis Queue
REDIS_QUEUE=default

# RabbitMQ Configuration
RABBITMQ_HOST=127.0.0.1
RABBITMQ_PORT=5672
RABBITMQ_USER=guest
RABBITMQ_PASSWORD=guest
RABBITMQ_VHOST=/

# Kafka Configuration
KAFKA_BROKERS=127.0.0.1:9092
KAFKA_GROUP_ID=backoffice-service
KAFKA_TOPIC_PREFIX=backoffice

# ============================================
# Logging Configuration
# ============================================
LOG_CHANNEL=stdout
# LOG_CHANNEL=file
# LOG_CHANNEL=stack

LOG_LEVEL=debug
# LOG_LEVEL=info
# LOG_LEVEL=warn
# LOG_LEVEL=error

LOG_FILE=storage/logs/app.log
LOG_MAX_SIZE=10
LOG_MAX_BACKUPS=5
LOG_MAX_AGE=28

# ============================================
# CORS Configuration
# ============================================
CORS_ALLOWED_ORIGINS=*
CORS_ALLOWED_METHODS=GET,POST,PUT,DELETE,OPTIONS
CORS_ALLOWED_HEADERS=*
CORS_ALLOW_CREDENTIALS=true

# ============================================
# Rate Limiting Configuration
# ============================================
RATE_LIMIT_ENABLED=true
RATE_LIMIT_MAX_REQUESTS=60
RATE_LIMIT_WINDOW=1m

# ============================================
# Cache Configuration
# ============================================
CACHE_DRIVER=memory
# CACHE_DRIVER=redis
# CACHE_DRIVER=file

CACHE_PREFIX=backoffice_cache
CACHE_TTL=3600

# ============================================
# Session Configuration
# ============================================
SESSION_DRIVER=memory
# SESSION_DRIVER=redis
# SESSION_DRIVER=cookie

SESSION_LIFETIME=120
SESSION_ENCRYPT=false
SESSION_PATH=/
SESSION_DOMAIN=

# ============================================
# API Configuration
# ============================================
API_PREFIX=/api/v1
API_RATE_LIMIT=60

# ============================================
# Third-Party Services (Optional)
# ============================================

# Video story generation (OpenAI-compatible API)
OPENAI_API_KEY=
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_MODEL=gpt-4o-mini

# Sentry Error Tracking
SENTRY_DSN=
SENTRY_ENVIRONMENT=production

# New Relic APM
NEW_RELIC_LICENSE_KEY=
NEW_RELIC_APP_NAME="${APP_NAME}"

# ============================================
# Development/Testing
# ============================================
# Set to true to enable debug mode
DEBUG=true

# Seed database on startup (development only)
DB_SEED=false

# Run migrations on startup
DB_MIGRATE=false
```

## Instructions

1. Create a file named `.env.example` in the root directory
2. Copy the content above into the file
3. Copy `.env.example` to `.env` for your local development
4. Update the values in `.env` with your actual configuration

