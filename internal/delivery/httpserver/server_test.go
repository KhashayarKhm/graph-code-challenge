package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"graph-code-challenge/internal/config"
	"graph-code-challenge/internal/delivery/httpserver/taskhandler"
)

type metricsStub struct{}

func (metricsStub) ObserveRequest(string, string, string, time.Duration) {}

func (metricsStub) Handler() http.Handler {
	return http.NotFoundHandler()
}

func TestSwaggerEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := New(
		config.Config{AppMode: config.AppModeTest, HTTPPort: 8080},
		taskhandler.Handler{},
		metricsStub{},
	)
	server.Setup()

	t.Run("UI", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if contentType := recorder.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
			t.Fatalf("Content-Type = %q, want HTML", contentType)
		}
	})

	t.Run("spec", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil))

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}

		var spec struct {
			Swagger string                     `json:"swagger"`
			Paths   map[string]json.RawMessage `json:"paths"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &spec); err != nil {
			t.Fatalf("decode spec: %v", err)
		}
		if spec.Swagger != "2.0" {
			t.Fatalf("swagger = %q, want %q", spec.Swagger, "2.0")
		}
		for _, path := range []string{"/healthz", "/api/v1/tasks", "/api/v1/tasks/{id}"} {
			if _, ok := spec.Paths[path]; !ok {
				t.Errorf("generated spec is missing path %q", path)
			}
		}
	})
}
