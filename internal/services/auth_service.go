package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/app/models"
	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/pkg/database"
	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/shared/logger"
	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/shared/utils"

	"github.com/wasitmirani/linqflow-backend-golang-apis/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuthService handles authentication business logic
type AuthService struct {
	db     *database.Manager
	config *config.Config
	logger logger.Logger
}

// NewAuthService creates a new auth service
func NewAuthService(db *database.Manager, cfg *config.Config, log logger.Logger) *AuthService {
	return &AuthService{
		db:     db,
		config: cfg,
		logger: log,
	}
}

// Login authenticates a user with email and password
func (s *AuthService) Login(ctx context.Context, email, password string) (map[string]interface{}, error) {
	// Get primary database
	primaryDriver, err := s.db.GetDriver("primary")
	if err != nil {
		return nil, fmt.Errorf("database connection error: %w", err)
	}

	var user models.User

	// Check if using GORM
	if gormDB := primaryDriver.GetGormDB(); gormDB != nil {
		db := gormDB.(*gorm.DB)
		if err := db.WithContext(ctx).Where("email = ? AND active = ?", email, true).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("invalid credentials")
			}
			return nil, fmt.Errorf("database error: %w", err)
		}
	} else {
		// Use raw SQL
		sqlDB := primaryDriver.GetSQLDB()
		query := `SELECT id, email, username, password, first_name, last_name, role, active, created_at, updated_at 
		          FROM users WHERE email = $1 AND active = $2`

		err := sqlDB.QueryRowContext(ctx, query, email, true).Scan(
			&user.ID, &user.Email, &user.Username, &user.Password,
			&user.FirstName, &user.LastName, &user.Role, &user.Active,
			&user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, errors.New("invalid credentials")
			}
			return nil, fmt.Errorf("database error: %w", err)
		}
	}

	// Verify password
	if !utils.CheckPasswordHash(password, user.Password) {
		return nil, errors.New("invalid credentials")
	}

	// Generate JWT token
	token, err := s.generateToken(user.ID.String(), user.Email, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Remove password from response
	user.Password = ""

	return map[string]interface{}{
		"token": token,
		"user":  user,
	}, nil
}

// Register registers a new user
func (s *AuthService) Register(ctx context.Context, req interface{}) (*models.User, error) {
	// Type assert request
	registerReq, ok := req.(map[string]interface{})
	if !ok {
		// Try to get from struct if needed
		return nil, errors.New("invalid request format")
	}

	email, _ := registerReq["email"].(string)
	password, _ := registerReq["password"].(string)
	firstName, _ := registerReq["first_name"].(string)
	lastName, _ := registerReq["last_name"].(string)
	username, _ := registerReq["username"].(string)

	if email == "" || password == "" {
		return nil, errors.New("email and password are required")
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Get primary database
	primaryDriver, err := s.db.GetDriver("primary")
	if err != nil {
		return nil, fmt.Errorf("database connection error: %w", err)
	}

	user := models.User{
		ID:        uuid.New(),
		Email:     email,
		Username:  username,
		Password:  hashedPassword,
		FirstName: firstName,
		LastName:  lastName,
		Role:      models.RoleUser,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Check if using GORM
	if gormDB := primaryDriver.GetGormDB(); gormDB != nil {
		db := gormDB.(*gorm.DB)
		if err := db.WithContext(ctx).Create(&user).Error; err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	} else {
		// Use raw SQL
		sqlDB := primaryDriver.GetSQLDB()
		query := `INSERT INTO users (id, email, username, password, first_name, last_name, role, active, created_at, updated_at)
		          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

		_, err := sqlDB.ExecContext(ctx, query,
			user.ID, user.Email, user.Username, user.Password,
			user.FirstName, user.LastName, user.Role, user.Active,
			user.CreatedAt, user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	// Remove password from response
	user.Password = ""

	return &user, nil
}

// RefreshToken refreshes an access token
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (map[string]interface{}, error) {
	// Parse and validate refresh token
	claims, err := utils.VerifyToken(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	email, ok := (*claims)["email"].(string)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	// Generate new access token
	userID, _ := (*claims)["user_id"].(string)
	role, _ := (*claims)["role"].(string)

	token, err := s.generateToken(userID, email, role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return map[string]interface{}{
		"token": token,
	}, nil
}

// Logout logs out a user (can be extended to blacklist tokens)
func (s *AuthService) Logout(ctx context.Context, token string) error {
	// TODO: Implement token blacklisting if needed
	// For now, just return success
	return nil
}

// generateToken generates a JWT token
func (s *AuthService) generateToken(userID, email, role string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"role":    role,
		"exp":     time.Now().Add(s.config.JWT.Expiration).Unix(),
		"iat":     time.Now().Unix(),
		"iss":     s.config.JWT.Issuer,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWT.Secret))
}

// Health checks if the service is healthy
func (s *AuthService) Health(ctx context.Context) error {
	return s.db.Health(ctx)["primary"]
}
