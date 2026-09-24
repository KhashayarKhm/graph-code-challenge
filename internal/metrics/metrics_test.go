package metrics_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"graph-code-challenge/internal/metrics"
)

type counterStub struct {
	count int64
	err   error
}

func (c counterStub) Count(context.Context) (int64, error) {
	return c.count, c.err
}

func scrape(t *testing.T, m *metrics.Metrics) string {
	t.Helper()

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("scrape status = %d, want %d", rec.Code, http.StatusOK)
	}

	return rec.Body.String()
}

func requireContains(t *testing.T, body, want string) {
	t.Helper()

	if !strings.Contains(body, want) {
		t.Errorf("scrape output is missing %q\ngot:\n%s", want, body)
	}
}

func TestObserveRequestExposesCounterAndHistogram(t *testing.T) {
	m := metrics.New()

	m.ObserveRequest(http.MethodGet, "/api/v1/tasks", "200", 30*time.Millisecond)
	m.ObserveRequest(http.MethodGet, "/api/v1/tasks", "200", 70*time.Millisecond)
	m.ObserveRequest(http.MethodPost, "/api/v1/tasks", "201", 10*time.Millisecond)

	body := scrape(t, m)

	requireContains(t, body, `http_requests_total{method="GET",path="/api/v1/tasks",status="200"} 2`)
	requireContains(t, body, `http_requests_total{method="POST",path="/api/v1/tasks",status="201"} 1`)
	requireContains(t, body, `http_request_duration_seconds_count{method="GET",path="/api/v1/tasks"} 2`)
	requireContains(t, body, `http_request_duration_seconds_sum{method="GET",path="/api/v1/tasks"} 0.1`)
	requireContains(t, body, `http_request_duration_seconds_bucket{method="GET",path="/api/v1/tasks",le="0.05"} 1`)
}

func TestRequestMetricsAreTypedCorrectly(t *testing.T) {
	m := metrics.New()
	m.ObserveRequest(http.MethodGet, "/healthz", "200", time.Millisecond)

	body := scrape(t, m)

	requireContains(t, body, "# TYPE http_requests_total counter")
	requireContains(t, body, "# TYPE http_request_duration_seconds histogram")
}

func TestTaskGaugeIsExposed(t *testing.T) {
	m := metrics.New()

	if err := m.RegisterTaskGauge(counterStub{count: 17}); err != nil {
		t.Fatalf("RegisterTaskGauge: %v", err)
	}

	body := scrape(t, m)

	requireContains(t, body, "# TYPE tasks_total gauge")
	requireContains(t, body, "tasks_total 17")
}

func TestTaskGaugeIsReadOnEveryScrape(t *testing.T) {
	m := metrics.New()
	stub := &mutableCounter{count: 1}

	if err := m.RegisterTaskGauge(stub); err != nil {
		t.Fatalf("RegisterTaskGauge: %v", err)
	}

	requireContains(t, scrape(t, m), "tasks_total 1")

	stub.count = 2

	requireContains(t, scrape(t, m), "tasks_total 2")
}

func TestTaskGaugeReportsNaNWhenTheQueryFails(t *testing.T) {
	m := metrics.New()

	if err := m.RegisterTaskGauge(counterStub{err: errors.New("database is down")}); err != nil {
		t.Fatalf("RegisterTaskGauge: %v", err)
	}

	requireContains(t, scrape(t, m), "tasks_total NaN")
}

func TestRuntimeCollectorsAreRegistered(t *testing.T) {
	body := scrape(t, metrics.New())

	requireContains(t, body, "go_goroutines")
	requireContains(t, body, "go_memstats_alloc_bytes")
}

type mutableCounter struct {
	count int64
}

func (c *mutableCounter) Count(context.Context) (int64, error) {
	return c.count, nil
}
