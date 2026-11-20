package user

import (
	"net/http"
	"strconv"

	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/shared/errors"
	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/services"

	"github.com/gin-gonic/gin"
)

// UserController handles user-related HTTP requests
type UserController struct {
	userService *services.UserService
}

// NewUserController creates a new user controller
func NewUserController(userService *services.UserService) *UserController {
	return &UserController{
		userService: userService,
	}
}

// GetUser handles getting a user by ID
// @Summary Get user by ID
// @Description Get user details by ID
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/users/{id} [get]
func (uc *UserController) GetUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		appErr := errors.NewBadRequestError("User ID is required", nil)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	user, err := uc.userService.GetUser(c.Request.Context(), id)
	if err != nil {
		appErr := errors.NewNotFoundError("User not found", err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": user,
	})
}

// ListUsers handles listing users with pagination
// @Summary List users
// @Description Get a list of users with pagination
// @Tags users
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/users [get]
func (uc *UserController) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	users, err := uc.userService.ListUsers(c.Request.Context(), limit, offset)
	if err != nil {
		appErr := errors.NewInternalServerError("Failed to fetch users", err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": users,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": len(users),
		},
	})
}

// CreateUser handles creating a new user
// @Summary Create user
// @Description Create a new user
// @Tags users
// @Accept json
// @Produce json
// @Param user body map[string]interface{} true "User data"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/users [post]
func (uc *UserController) CreateUser(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := errors.NewValidationError("Invalid request data", err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	user, err := uc.userService.CreateUser(c.Request.Context(), req)
	if err != nil {
		appErr := errors.NewInternalServerError("Failed to create user", err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"data":    user,
	})
}

// UpdateUser handles updating a user
// @Summary Update user
// @Description Update user by ID
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param user body map[string]interface{} true "User data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/users/{id} [put]
func (uc *UserController) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		appErr := errors.NewBadRequestError("User ID is required", nil)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := errors.NewValidationError("Invalid request data", err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	user, err := uc.userService.UpdateUser(c.Request.Context(), id, req)
	if err != nil {
		appErr := errors.NewNotFoundError("User not found", err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User updated successfully",
		"data":    user,
	})
}

// DeleteUser handles deleting a user
// @Summary Delete user
// @Description Delete user by ID
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/users/{id} [delete]
func (uc *UserController) DeleteUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		appErr := errors.NewBadRequestError("User ID is required", nil)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	err := uc.userService.DeleteUser(c.Request.Context(), id)
	if err != nil {
		appErr := errors.NewNotFoundError("User not found", err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User deleted successfully",
	})
}
