package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"graph-code-challenge/internal/delivery/httpserver/middleware"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

type observation struct {
	method   string
	path     string
	status   string
	duration time.Duration
}

type recorderStub struct {
	observations []observation
}

func (r *recorderStub) ObserveRequest(method, path, status string, d time.Duration) {
	r.observations = append(r.observations, observation{method, path, status, d})
}

func serve(recorder middleware.Recorder, method, target string, register func(*gin.Engine)) {
	router := gin.New()
	router.Use(middleware.Metrics(recorder))
	register(router)

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, target, nil))
}

func TestMetricsRecordsMatchedRoute(t *testing.T) {
	testCases := []struct {
		name       string
		method     string
		target     string
		register   func(*gin.Engine)
		wantMethod string
		wantPath   string
		wantStatus string
	}{
		{
			name:   "static route",
			method: http.MethodGet,
			target: "/healthz",
			register: func(r *gin.Engine) {
				r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
			},
			wantMethod: http.MethodGet,
			wantPath:   "/healthz",
			wantStatus: "200",
		},
		{
			name:   "parameterised route reports the template, not the value",
			method: http.MethodGet,
			target: "/api/v1/tasks/42",
			register: func(r *gin.Engine) {
				r.GET("/api/v1/tasks/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
			},
			wantMethod: http.MethodGet,
			wantPath:   "/api/v1/tasks/:id",
			wantStatus: "200",
		},
		{
			name:   "error status is recorded",
			method: http.MethodPost,
			target: "/api/v1/tasks",
			register: func(r *gin.Engine) {
				r.POST("/api/v1/tasks", func(c *gin.Context) { c.Status(http.StatusUnprocessableEntity) })
			},
			wantMethod: http.MethodPost,
			wantPath:   "/api/v1/tasks",
			wantStatus: "422",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &recorderStub{}
			serve(stub, tc.method, tc.target, tc.register)

			if len(stub.observations) != 1 {
				t.Fatalf("got %d observations, want 1", len(stub.observations))
			}

			got := stub.observations[0]

			if got.method != tc.wantMethod {
				t.Errorf("method = %q, want %q", got.method, tc.wantMethod)
			}

			if got.path != tc.wantPath {
				t.Errorf("path = %q, want %q", got.path, tc.wantPath)
			}

			if got.status != tc.wantStatus {
				t.Errorf("status = %q, want %q", got.status, tc.wantStatus)
			}

			if got.duration < 0 {
				t.Errorf("duration = %s, want a non-negative value", got.duration)
			}
		})
	}
}

func TestMetricsCollapsesUnmatchedRoutes(t *testing.T) {
	stub := &recorderStub{}

	serve(stub, http.MethodGet, "/no/such/route", func(*gin.Engine) {})

	if len(stub.observations) != 1 {
		t.Fatalf("got %d observations, want 1", len(stub.observations))
	}

	if got := stub.observations[0].path; got != "unmatched" {
		t.Errorf("path = %q, want %q", got, "unmatched")
	}

	if got := stub.observations[0].status; got != "404" {
		t.Errorf("status = %q, want %q", got, "404")
	}
}

func TestMetricsObservesElapsedTime(t *testing.T) {
	stub := &recorderStub{}
	const sleep = 10 * time.Millisecond

	serve(stub, http.MethodGet, "/slow", func(r *gin.Engine) {
		r.GET("/slow", func(c *gin.Context) {
			time.Sleep(sleep)
			c.Status(http.StatusOK)
		})
	})

	if got := stub.observations[0].duration; got < sleep {
		t.Errorf("duration = %s, want at least %s", got, sleep)
	}
}
