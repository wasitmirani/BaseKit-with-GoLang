package auth

import (
	"net/http"

	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/shared/errors"
	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/services"

	"github.com/gin-gonic/gin"
)

// AuthController handles authentication-related HTTP requests
type AuthController struct {
	authService *services.AuthService
}

// NewAuthController creates a new auth controller
func NewAuthController(authService *services.AuthService) *AuthController {
	return &AuthController{
		authService: authService,
	}
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// RegisterRequest represents the registration request payload
type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=6"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	Username  string `json:"username" binding:"required"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Token string      `json:"token"`
	User  interface{} `json:"user"`
}

// Register handles user registration
// @Summary Register a new user
// @Description Register a new user account
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration data"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/auth/register [post]
func (ac *AuthController) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := errors.NewValidationError("Invalid request data", err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	user, err := ac.authService.Register(c.Request.Context(), &req)
	if err != nil {
		appErr := errors.NewInternalServerError("Failed to register user", err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user":    user,
	})
}

// Login handles user authentication
// @Summary Login user
// @Description Authenticate user and return JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/v1/auth/login [post]
func (ac *AuthController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := errors.NewValidationError("Invalid request data", err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	result, err := ac.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		appErr := errors.NewUnauthorizedError("Invalid credentials", err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Logout handles user logout
// @Summary Logout user
// @Description Logout user and invalidate token
// @Tags auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/auth/logout [post]
func (ac *AuthController) Logout(c *gin.Context) {
	// Get token from header
	token := c.GetHeader("Authorization")
	if token != "" {
		// Invalidate token (add to blacklist, etc.)
		_ = ac.authService.Logout(c.Request.Context(), token)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// RefreshToken handles token refresh
// @Summary Refresh JWT token
// @Description Refresh an expired JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param refresh_token body string true "Refresh token"
// @Success 200 {object} LoginResponse
// @Failure 401 {object} map[string]interface{}
// @Router /api/v1/auth/refresh [post]
func (ac *AuthController) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := errors.NewValidationError("Invalid request data", err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	result, err := ac.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		appErr := errors.NewUnauthorizedError("Invalid refresh token", err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusOK, result)
}
