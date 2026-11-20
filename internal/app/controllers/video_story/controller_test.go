package video_story

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/wasitmirani/linqflow-backend-golang-apis/config"
	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/services"
	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/shared/logger"
)

func TestCreateValidatesVideoURLAndProviderConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	controller := NewController(
		services.NewVideoStoryService(config.VideoStoryConfig{}),
		logger.NewSimpleLogger(),
	)
	router.POST("/api/v1/video-stories", controller.Create)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "missing URL", body: `{}`, wantStatus: http.StatusBadRequest},
		{name: "unsupported URL", body: `{"video_url":"https://example.com/watch?v=abcdefghijk"}`, wantStatus: http.StatusBadRequest},
		{name: "provider not configured", body: `{"video_url":"https://youtu.be/abcdefghijk"}`, wantStatus: http.StatusServiceUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/video-stories", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("got status %d, want %d; body=%s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
		})
	}
}
