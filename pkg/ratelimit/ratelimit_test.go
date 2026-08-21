package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestRouter(publicPerMinute, authPerMinute int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Middleware(publicPerMinute, authPerMinute))
	router.GET("/resource", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return router
}

func performRequest(router http.Handler, path, authorization string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.RemoteAddr = "192.0.2.10:12345"
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestMiddlewareEnforcesSeparatePublicAndAuthenticatedLimits(t *testing.T) {
	router := newTestRouter(1, 2)

	if status := performRequest(router, "/resource", "").Code; status != http.StatusNoContent {
		t.Fatalf("first public request status = %d, want %d", status, http.StatusNoContent)
	}
	if status := performRequest(router, "/resource", "").Code; status != http.StatusTooManyRequests {
		t.Fatalf("second public request status = %d, want %d", status, http.StatusTooManyRequests)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if status := performRequest(router, "/resource", "Bearer demo-token").Code; status != http.StatusNoContent {
			t.Fatalf("authenticated request %d status = %d, want %d", attempt, status, http.StatusNoContent)
		}
	}
	if status := performRequest(router, "/resource", "Bearer demo-token").Code; status != http.StatusTooManyRequests {
		t.Fatalf("third authenticated request status = %d, want %d", status, http.StatusTooManyRequests)
	}
}

func TestMiddlewareExemptsHealthEndpoint(t *testing.T) {
	router := newTestRouter(1, 1)

	if status := performRequest(router, "/resource", "").Code; status != http.StatusNoContent {
		t.Fatalf("resource request status = %d, want %d", status, http.StatusNoContent)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		if status := performRequest(router, "/health", "").Code; status != http.StatusOK {
			t.Fatalf("health request %d status = %d, want %d", attempt, status, http.StatusOK)
		}
	}
}
