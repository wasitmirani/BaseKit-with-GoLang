package video_story

import (
	"errors"
	"net/http"
	"strings"

	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/services"
	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/shared/logger"

	"github.com/gin-gonic/gin"
)

// Controller handles video-to-story requests.
type Controller struct {
	service services.VideoStoryCreator
	logger  logger.Logger
}

// NewController creates a video story controller.
func NewController(service services.VideoStoryCreator, log logger.Logger) *Controller {
	return &Controller{service: service, logger: log}
}

// Create generates an original short-form script from a YouTube video.
func (c *Controller) Create(ctx *gin.Context) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1<<20)

	var request services.VideoStoryRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if strings.TrimSpace(request.VideoURL) == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "video_url is required"})
		return
	}

	result, err := c.service.Create(ctx.Request.Context(), request)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidYouTubeURL):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "video_url must be a supported YouTube URL"})
		case errors.Is(err, services.ErrTranscriptUnavailable):
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Video captions are unavailable"})
		case errors.Is(err, services.ErrStoryProviderNotConfigured):
			ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Video story generation is not configured"})
		case errors.Is(err, services.ErrInvalidStoryPreference):
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "audience and tone must be 120 characters or fewer"})
		default:
			c.logger.Error("Video story generation failed", logger.Field{Key: "error", Value: err.Error()})
			ctx.JSON(http.StatusBadGateway, gin.H{"error": "Failed to generate video story"})
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": result})
}
