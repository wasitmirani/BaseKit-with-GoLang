# LinqFlow Backend APIs (Go)

LinqFlow Backend APIs is a Go 1.24 service built with Gin. It provides clean architecture, multi-database support, daily file logging, messaging and storage integrations, and a GitHub Actions CI/CD pipeline.

## Features

- **Gin Framework** — High-performance HTTP web framework for Go
- **Clean Architecture** — Controller → Service → Database pattern
- **Multi-Database Support** — PostgreSQL and MySQL (GORM or raw SQL), designed for more drivers
- **Daily File Logging** — stdout, file, or stack with daily/size-based rotation
- **JWT Authentication** — Token-based auth with bcrypt password hashing
- **Modular Config** — Split config packages for app, auth, cache, database, queue, session, and services
- **Shared Utilities** — Common helpers, enums, errors, logger, JWT/hash utils, and validation under `internal/shared`
- **Infrastructure Ready** — Redis, email, Kafka, RabbitMQ, S3, and Azure Blob client stubs
- **Workers & WebSockets** — Placeholders for background jobs and real-time features
- **Docker & Kubernetes** — Container and K8s manifests under `deployments/`
- **CI/CD Pipeline** — GitHub Actions for CI, CD, and releases
- **Health Checks** — Built-in `/health` and `/ready` endpoints
- **Video-to-Story API** — Turn public YouTube captions into an original, scene-by-scene short-form video script
- **Graceful Shutdown** — Proper application lifecycle management

## Project Structure

```
.
├── cmd/
│   └── main.go                 # Application entry point
├── config/                     # Configuration (Viper + .env)
│   ├── config.go               # Root config loader
│   ├── app.go                  # App settings
│   ├── auth.go                 # Auth / JWT
│   ├── cache.go                # Cache
│   ├── database.go             # Database
│   ├── queue.go                # Queue / messaging
│   ├── session.go              # Session
│   └── services.go             # Third-party services
├── internal/
│   ├── app/                    # Application layer
│   │   ├── app.go              # App bootstrap, DI, routes wiring
│   │   ├── controllers/        # HTTP controllers (auth, user)
│   │   └── models/             # Domain models
│   ├── services/               # Business logic
│   ├── routes/                 # Route registration
│   ├── pkg/                    # Core packages
│   │   ├── constants/          # App constants
│   │   ├── database/           # DB drivers, factory, manager
│   │   ├── logger/             # Logging (stdout / file / stack)
│   │   ├── errors/             # App error types
│   │   ├── utils/              # JWT, hashing
│   │   ├── validator/          # Request validation
│   │   └── service/            # Service interfaces
│   ├── shared/                 # Shared cross-cutting code
│   │   ├── common/             # Pagination and shared DTOs
│   │   ├── enums/              # Domain enums
│   │   ├── errors/             # Shared errors
│   │   ├── logger/             # Shared logger helpers
│   │   ├── utils/              # Shared utilities
│   │   └── validator/          # Shared validation
│   ├── infrastructure/         # External integrations
│   │   ├── email/
│   │   ├── messaging/          # Kafka, RabbitMQ
│   │   ├── redis/
│   │   └── storage/            # S3, Azure
│   ├── database/               # Migrations & seeders
│   ├── storage/                # Local public/private file storage
│   ├── templates/              # Email templates
│   ├── websocket/              # WebSocket handlers
│   ├── workers/                # Background workers
│   └── docs/                   # API docs (Swagger)
├── deployments/
│   ├── docker/                 # docker-compose
│   └── kubernetes/            # Deployment, service, ingress
├── scripts/                    # deploy, migrate, seed
├── tests/                      # Integration / package tests
├── .github/workflows/          # CI, CD, release
├── .env.example                # Environment template
├── Dockerfile
├── Makefile
└── go.mod
```

## Getting Started

### Prerequisites

- Go 1.24 or higher
- PostgreSQL or MySQL
- Git
- Docker (optional)

### Installation

1. **Clone the repository**

   ```bash
   git clone https://github.com/wasitmirani/linqflow-backend-golang-apis.git
   cd linqflow-backend-golang-apis
   ```

2. **Set up environment variables**

   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

3. **Install dependencies**

   ```bash
   go mod download
   go mod tidy
   ```

4. **Configure database**

   - Update database credentials in `.env`
   - Optionally enable `DB_MIGRATE=true` / `DB_SEED=true` for local setup

5. **Run the application**

   ```bash
   # Development mode
   make dev

   # Or directly
   go run ./cmd/main.go
   ```

The API listens on `http://localhost:8080` by default.

## Configuration

Key options (see [`.env.example`](.env.example) or [`ENV_EXAMPLE.md`](ENV_EXAMPLE.md) for the full list):

```bash
# Application
APP_NAME="Backoffice Service"
APP_ENV=local
APP_DEBUG=true

# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
GIN_MODE=debug

# Database
DB_DRIVER=postgresql
DB_HOST=127.0.0.1
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=backoffice
DB_USE_GORM=true

# JWT
JWT_SECRET=your-secret-key-min-32-chars
JWT_EXPIRATION=24h

# Logging
LOG_CHANNEL=file          # stdout, file, or stack
LOG_LEVEL=debug
LOG_FILE_PATH=./storage/logs
LOG_FILE_NAME=app
LOG_DAILY_ROTATE=true
LOG_MAX_SIZE=10
LOG_MAX_BACKUPS=5
LOG_MAX_AGE=28

# Video story generation (OpenAI-compatible API)
OPENAI_API_KEY=your-api-key
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_MODEL=gpt-4o-mini
```

### Generate a video story

`POST /api/v1/video-stories` accepts a public YouTube URL. The service reads available
captions and uses the configured OpenAI-compatible chat-completions API to return an
analysis and an original short-form script. Videos without accessible captions, or
requests made without `OPENAI_API_KEY`, return an explicit error.

```json
{
  "video_url": "https://www.youtube.com/watch?v=49_Ph2q6uIM",
  "audience": "general audience",
  "tone": "curious and cinematic"
}
```

The response contains the video title, summary, key points, hook, scene-by-scene
visuals, narration and on-screen text, plus a closing call to action. Configure
`SERVER_WRITE_TIMEOUT` to allow time for upstream caption and AI requests; its
default is 90 seconds.

### Logging

| Channel | Behavior |
|---------|----------|
| `stdout` | Console only (good for local dev) |
| `file` | Rotated log files (`app-YYYY-MM-DD.log` when daily rotate is on) |
| `stack` | Both stdout and file |

## API Endpoints

### Authentication

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/auth/register` | Register a new user |
| `POST` | `/api/v1/auth/login` | Login |
| `POST` | `/api/v1/auth/logout` | Logout |
| `POST` | `/api/v1/auth/refresh` | Refresh JWT |

### Users

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/users` | List users (paginated) |
| `GET` | `/api/v1/users/:id` | Get user by ID |
| `POST` | `/api/v1/users` | Create user |
| `PUT` | `/api/v1/users/:id` | Update user |
| `DELETE` | `/api/v1/users/:id` | Delete user |

### Health

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness |
| `GET` | `/ready` | Readiness |

## Architecture

```
HTTP Request
    ↓
Controller (HTTP handling, validation)
    ↓
Service (Business logic)
    ↓
Database Driver (GORM or raw SQL)
```

### Database drivers

- **PostgreSQL** — GORM and raw SQL
- **MySQL** — GORM and raw SQL
- **Extensible** — Add MongoDB, SQLite, etc. via the driver interface

Optional secondary databases can be configured in `.env` (`DB_SECONDARY_*`).

## Docker

```bash
# Build
make docker-build
# or
docker build -t backoffice-service:latest .

# Run
make docker-run
# or
docker run -p 8080:8080 --env-file .env backoffice-service:latest

# Compose
cd deployments/docker
docker-compose up -d
```

## Testing

```bash
make test              # All tests + coverage HTML
make test-coverage     # Coverage summary
make test-verbose      # Verbose
```

## Code Quality

```bash
make lint              # golangci-lint
make lint-fix         # Auto-fix where possible
make security          # gosec scan
make fmt               # Format
```

## CI/CD

GitHub Actions workflows cover:

- **CI** — test, lint, build, security scan
- **CD** — Docker build and deploy
- **Release** — versioned binaries / images

See [CI_CD.md](CI_CD.md) and [`.github/workflows/README.md`](.github/workflows/README.md).

## Makefile

```bash
make help            # List targets
make build           # Build binary to bin/
make run             # Run app
make test            # Tests
make lint            # Linters
make docker-build    # Docker image
make docker-run      # Run container
make dev             # Dev mode (stdout debug logs)
make deps            # Download & tidy modules
make migrate         # Run migrations
make seed            # Seed data
make install-tools   # Install golangci-lint, gosec, etc.
```

## Security

- JWT authentication
- bcrypt password hashing
- Input validation
- gosec in CI / `make security`
- Non-root Docker user

## Documentation

| Doc | Description |
|-----|-------------|
| [ARCHITECTURE.md](ARCHITECTURE.md) | Architecture overview |
| [CI_CD.md](CI_CD.md) | CI/CD details |
| [LOGGING_SETUP.md](LOGGING_SETUP.md) | Logging setup |
| [ENV_EXAMPLE.md](ENV_EXAMPLE.md) | Env var reference |
| [STRUCTURE_OPTIMIZATION.md](STRUCTURE_OPTIMIZATION.md) | Structure notes |
| [UPDATES.md](UPDATES.md) | Change summary |

## Roadmap

- [ ] Complete MongoDB / SQLite drivers
- [ ] Wire Redis caching into the request path
- [ ] Rate limiting middleware
- [ ] Finish Swagger / OpenAPI from `internal/docs`
- [ ] Database migration runner integration
- [ ] Monitoring and metrics (Prometheus / OpenTelemetry)
- [ ] Flesh out workers and WebSocket handlers

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit (`git commit -m 'feat: add amazing feature'`)
4. Push (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Commit message format

Follow [Conventional Commits](https://conventionalcommits.org/):

```
<type>(<scope>): <subject>
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`

## Author

Wasit Mirani — [GitHub](https://github.com/wasitmirani)

## Acknowledgments

- [Gin](https://github.com/gin-gonic/gin)
- [GORM](https://gorm.io/)
- [Viper](https://github.com/spf13/viper)
- The Go community

---

Star this repo if you find it useful.
