package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/app/models"
	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/pkg/database"
	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/shared/logger"
	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/shared/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserService handles user business logic
type UserService struct {
	db     *database.Manager
	logger logger.Logger
}

// NewUserService creates a new user service
func NewUserService(db *database.Manager, log logger.Logger) *UserService {
	return &UserService{
		db:     db,
		logger: log,
	}
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, id string) (*models.User, error) {
	// Get primary database
	primaryDriver, err := s.db.GetDriver("primary")
	if err != nil {
		return nil, fmt.Errorf("database connection error: %w", err)
	}

	var user models.User
	userID, err := uuid.Parse(id)
	if err != nil {
		return nil, errors.New("invalid user ID format")
	}

	// Check if using GORM
	if gormDB := primaryDriver.GetGormDB(); gormDB != nil {
		db := gormDB.(*gorm.DB)
		if err := db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("user not found")
			}
			return nil, fmt.Errorf("database error: %w", err)
		}
	} else {
		// Use raw SQL
		sqlDB := primaryDriver.GetSQLDB()
		query := `SELECT id, email, username, password, first_name, last_name, role, active, created_at, updated_at 
		          FROM users WHERE id = $1`

		err := sqlDB.QueryRowContext(ctx, query, userID).Scan(
			&user.ID, &user.Email, &user.Username, &user.Password,
			&user.FirstName, &user.LastName, &user.Role, &user.Active,
			&user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, errors.New("user not found")
			}
			return nil, fmt.Errorf("database error: %w", err)
		}
	}

	// Remove password from response
	user.Password = ""
	return &user, nil
}

// CreateUser creates a new user
func (s *UserService) CreateUser(ctx context.Context, req interface{}) (*models.User, error) {
	reqMap, ok := req.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid request format")
	}

	email, _ := reqMap["email"].(string)
	username, _ := reqMap["username"].(string)
	firstName, _ := reqMap["first_name"].(string)
	lastName, _ := reqMap["last_name"].(string)
	password, _ := reqMap["password"].(string)

	if email == "" {
		return nil, errors.New("email is required")
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
		FirstName: firstName,
		LastName:  lastName,
		Role:      models.RoleUser,
		Active:    true,
	}

	if password != "" {
		hashedPassword, err := utils.HashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		user.Password = hashedPassword
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
		          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())`

		_, err := sqlDB.ExecContext(ctx, query,
			user.ID, user.Email, user.Username, user.Password,
			user.FirstName, user.LastName, user.Role, user.Active,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	user.Password = ""
	return &user, nil
}

// UpdateUser updates an existing user
func (s *UserService) UpdateUser(ctx context.Context, id string, req interface{}) (*models.User, error) {
	user, err := s.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}

	reqMap, ok := req.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid request format")
	}

	// Update fields
	if email, ok := reqMap["email"].(string); ok && email != "" {
		user.Email = email
	}
	if username, ok := reqMap["username"].(string); ok && username != "" {
		user.Username = username
	}
	if firstName, ok := reqMap["first_name"].(string); ok && firstName != "" {
		user.FirstName = firstName
	}
	if lastName, ok := reqMap["last_name"].(string); ok && lastName != "" {
		user.LastName = lastName
	}
	if password, ok := reqMap["password"].(string); ok && password != "" {
		hashedPassword, err := utils.HashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		user.Password = hashedPassword
	}

	// Get primary database
	primaryDriver, err := s.db.GetDriver("primary")
	if err != nil {
		return nil, fmt.Errorf("database connection error: %w", err)
	}

	// Check if using GORM
	if gormDB := primaryDriver.GetGormDB(); gormDB != nil {
		db := gormDB.(*gorm.DB)
		if err := db.WithContext(ctx).Save(&user).Error; err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
	} else {
		// Use raw SQL
		sqlDB := primaryDriver.GetSQLDB()
		query := `UPDATE users SET email = $1, username = $2, first_name = $3, last_name = $4, 
		          password = $5, updated_at = NOW() WHERE id = $6`

		_, err := sqlDB.ExecContext(ctx, query,
			user.Email, user.Username, user.FirstName, user.LastName,
			user.Password, user.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
	}

	user.Password = ""
	return user, nil
}

// DeleteUser deletes a user by ID
func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	userID, err := uuid.Parse(id)
	if err != nil {
		return errors.New("invalid user ID format")
	}

	// Get primary database
	primaryDriver, err := s.db.GetDriver("primary")
	if err != nil {
		return fmt.Errorf("database connection error: %w", err)
	}

	// Check if using GORM
	if gormDB := primaryDriver.GetGormDB(); gormDB != nil {
		db := gormDB.(*gorm.DB)
		if err := db.WithContext(ctx).Delete(&models.User{}, userID).Error; err != nil {
			return fmt.Errorf("failed to delete user: %w", err)
		}
	} else {
		// Use raw SQL
		sqlDB := primaryDriver.GetSQLDB()
		query := `DELETE FROM users WHERE id = $1`

		_, err := sqlDB.ExecContext(ctx, query, userID)
		if err != nil {
			return fmt.Errorf("failed to delete user: %w", err)
		}
	}

	return nil
}

// ListUsers retrieves a list of users with pagination
func (s *UserService) ListUsers(ctx context.Context, limit, offset int) ([]*models.User, error) {
	// Get primary database
	primaryDriver, err := s.db.GetDriver("primary")
	if err != nil {
		return nil, fmt.Errorf("database connection error: %w", err)
	}

	var users []*models.User

	// Check if using GORM
	if gormDB := primaryDriver.GetGormDB(); gormDB != nil {
		db := gormDB.(*gorm.DB)
		if err := db.WithContext(ctx).Limit(limit).Offset(offset).Find(&users).Error; err != nil {
			return nil, fmt.Errorf("database error: %w", err)
		}
	} else {
		// Use raw SQL
		sqlDB := primaryDriver.GetSQLDB()
		query := `SELECT id, email, username, password, first_name, last_name, role, active, created_at, updated_at 
		          FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`

		rows, err := sqlDB.QueryContext(ctx, query, limit, offset)
		if err != nil {
			return nil, fmt.Errorf("database error: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var user models.User
			if err := rows.Scan(
				&user.ID, &user.Email, &user.Username, &user.Password,
				&user.FirstName, &user.LastName, &user.Role, &user.Active,
				&user.CreatedAt, &user.UpdatedAt,
			); err != nil {
				return nil, fmt.Errorf("failed to scan user: %w", err)
			}
			user.Password = ""
			users = append(users, &user)
		}
	}

	// Remove passwords from response
	for _, user := range users {
		user.Password = ""
	}

	return users, nil
}

// Health checks if the service is healthy
func (s *UserService) Health(ctx context.Context) error {
	return s.db.Health(ctx)["primary"]
}
