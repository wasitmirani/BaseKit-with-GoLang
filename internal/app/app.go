package app

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"BaseKit-with-GoLang/internal/app/controllers/auth"
	"BaseKit-with-GoLang/internal/app/controllers/user"
	"BaseKit-with-GoLang/internal/pkg/database"
	"BaseKit-with-GoLang/internal/shared/logger"
	"BaseKit-with-GoLang/internal/services"
	"BaseKit-with-GoLang/config"
)

// Application represents the main application
type Application struct {
	config    *config.Config
	logger    logger.Logger
	server    *http.Server
	router    *gin.Engine
	dbManager *database.Manager

	// Services
	authService *services.AuthService
	userService *services.UserService

	// Controllers
	authController *auth.AuthController
	userController *user.UserController
}

// New creates a new Application instance
func New(cfg *config.Config, log logger.Logger) (*Application, error) {
	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Create router
	router := gin.New()
	router.Use(gin.Recovery())

	// Add logging middleware
	router.Use(ginLogger(log))

	app := &Application{
		config:    cfg,
		logger:    log,
		router:    router,
		dbManager: database.NewManager(),
	}

	// Initialize database connections
	if err := app.initDatabase(); err != nil {
		return nil, err
	}

	// Initialize dependencies (services, controllers)
	if err := app.initDependencies(); err != nil {
		return nil, err
	}

	// Setup routes
	app.setupRoutes()

	// Create HTTP server
	app.server = &http.Server{
		Addr:         cfg.Server.Host + ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return app, nil
}

// initDatabase initializes database connections
func (app *Application) initDatabase() error {
	ctx := context.Background()

	// Initialize primary database
	driverType, driverConfig, err := app.config.Database.Primary.GetDatabaseDriverConfig()
	if err != nil {
		return err
	}

	factory := database.NewFactory()
	primaryDriver, err := factory.CreateDriver(driverType, driverConfig)
	if err != nil {
		return err
	}

	if err := app.dbManager.AddDriver("primary", primaryDriver); err != nil {
		return err
	}

	// Connect to primary database
	if err := primaryDriver.Connect(ctx); err != nil {
		return err
	}

	app.logger.Info("Primary database connected", logger.Field{Key: "driver", Value: driverType})

	// Initialize additional databases if configured
	for name, dbConfig := range app.config.Database.Databases {
		driverType, driverConfig, err := dbConfig.GetDatabaseDriverConfig()
		if err != nil {
			app.logger.Warn("Failed to get driver config", logger.Field{Key: "database", Value: name}, logger.Field{Key: "error", Value: err.Error()})
			continue
		}

		driver, err := factory.CreateDriver(driverType, driverConfig)
		if err != nil {
			app.logger.Warn("Failed to create driver", logger.Field{Key: "database", Value: name}, logger.Field{Key: "error", Value: err.Error()})
			continue
		}

		if err := app.dbManager.AddDriver(name, driver); err != nil {
			app.logger.Warn("Failed to add driver", logger.Field{Key: "database", Value: name}, logger.Field{Key: "error", Value: err.Error()})
			continue
		}

		if err := driver.Connect(ctx); err != nil {
			app.logger.Warn("Failed to connect driver", logger.Field{Key: "database", Value: name}, logger.Field{Key: "error", Value: err.Error()})
			continue
		}

		app.logger.Info("Database connected", logger.Field{Key: "name", Value: name}, logger.Field{Key: "driver", Value: driverType})
	}

	return nil
}

// initDependencies initializes services and controllers
func (app *Application) initDependencies() error {
	// Initialize services
	app.authService = services.NewAuthService(app.dbManager, app.config, app.logger)
	app.userService = services.NewUserService(app.dbManager, app.logger)
	app.logger.Info("Services initialized successfully")

	// Initialize controllers
	app.authController = auth.NewAuthController(app.authService)
	app.userController = user.NewUserController(app.userService)

	return nil
}

// setupRoutes sets up all application routes
func (app *Application) setupRoutes() {
	// Health check
	app.router.GET("/health", app.healthCheck)
	app.router.GET("/ready", app.readinessCheck)

	// API routes
	api := app.router.Group("/api/v1")
	{
		// Auth routes
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", app.authController.Register)
			authGroup.POST("/login", app.authController.Login)
			authGroup.POST("/logout", app.authController.Logout)
			authGroup.POST("/refresh", app.authController.RefreshToken)
		}

		// User routes
		usersGroup := api.Group("/users")
		{
			usersGroup.GET("", app.userController.ListUsers)
			usersGroup.GET("/:id", app.userController.GetUser)
			usersGroup.POST("", app.userController.CreateUser)
			usersGroup.PUT("/:id", app.userController.UpdateUser)
			usersGroup.DELETE("/:id", app.userController.DeleteUser)
		}
	}
}

// healthCheck handles health check requests
func (app *Application) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": app.config.App.Name,
		"version": app.config.App.Version,
		"uptime":  time.Since(startTime).String(),
	})
}

// readinessCheck handles readiness check requests
func (app *Application) readinessCheck(c *gin.Context) {
	ctx := c.Request.Context()
	healthResults := app.dbManager.Health(ctx)

	allHealthy := true
	for name, err := range healthResults {
		if err != nil {
			allHealthy = false
			app.logger.Error("Database health check failed", logger.Field{Key: "database", Value: name}, logger.Field{Key: "error", Value: err.Error()})
		}
	}

	if allHealthy {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ready",
			"databases": healthResults,
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":    "not ready",
			"databases": healthResults,
		})
	}
}

// Start starts the HTTP server
func (app *Application) Start() error {
	app.logger.Info("Starting server",
		logger.Field{Key: "host", Value: app.config.Server.Host},
		logger.Field{Key: "port", Value: app.config.Server.Port},
		logger.Field{Key: "mode", Value: app.config.Server.Mode},
	)
	return app.server.ListenAndServe()
}

// Shutdown gracefully shuts down the application
func (app *Application) Shutdown(ctx context.Context) error {
	app.logger.Info("Shutting down server...")

	// Close database connections
	if err := app.dbManager.CloseAll(); err != nil {
		app.logger.Error("Error closing database connections", logger.Field{Key: "error", Value: err.Error()})
	}

	// Shutdown HTTP server
	return app.server.Shutdown(ctx)
}

// GetRouter returns the Gin router (useful for testing)
func (app *Application) GetRouter() *gin.Engine {
	return app.router
}

// GetDBManager returns the database manager
func (app *Application) GetDBManager() *database.Manager {
	return app.dbManager
}

var startTime = time.Now()

// ginLogger creates a Gin middleware for logging
func ginLogger(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		if raw != "" {
			path = path + "?" + raw
		}

		if statusCode >= 500 {
			log.Error("HTTP Request",
				logger.Field{Key: "status", Value: statusCode},
				logger.Field{Key: "latency", Value: latency},
				logger.Field{Key: "client_ip", Value: clientIP},
				logger.Field{Key: "method", Value: method},
				logger.Field{Key: "path", Value: path},
				logger.Field{Key: "error", Value: errorMessage},
			)
		} else if statusCode >= 400 {
			log.Warn("HTTP Request",
				logger.Field{Key: "status", Value: statusCode},
				logger.Field{Key: "latency", Value: latency},
				logger.Field{Key: "client_ip", Value: clientIP},
				logger.Field{Key: "method", Value: method},
				logger.Field{Key: "path", Value: path},
			)
		} else {
			log.Info("HTTP Request",
				logger.Field{Key: "status", Value: statusCode},
				logger.Field{Key: "latency", Value: latency},
				logger.Field{Key: "client_ip", Value: clientIP},
				logger.Field{Key: "method", Value: method},
				logger.Field{Key: "path", Value: path},
			)
		}
	}
}
