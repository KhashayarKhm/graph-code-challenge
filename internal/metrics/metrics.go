package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type TaskCountReader interface {
	Count(ctx context.Context) (int64, error)
}

type Metrics struct {
	registry  *prometheus.Registry
	requests  *prometheus.CounterVec
	duration  *prometheus.HistogramVec
	taskCount prometheus.Gauge
}

func New() *Metrics {
	taskCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tasks_count",
		Help: "Number of tasks that have not been soft-deleted.",
	})

	m := &Metrics{
		registry: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "requests_total",
				Help: "Total number of HTTP requests served, by method, route and status code.",
			},
			[]string{"method", "path", "status"},
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "request_latency_histogram",
				Help:    "Duration of HTTP requests in seconds, by method and route.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		taskCount: taskCount,
	}

	m.registry.MustRegister(
		m.requests,
		m.duration,
		m.taskCount,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return m
}

func (m *Metrics) ObserveRequest(method, path, status string, d time.Duration) {
	m.requests.WithLabelValues(method, path, status).Inc()
	m.duration.WithLabelValues(method, path).Observe(d.Seconds())
}

func (m *Metrics) RefreshTaskGauge(ctx context.Context, reader TaskCountReader) error {
	count, err := reader.Count(ctx)
	if err != nil {
		return err
	}

	m.taskCount.Set(float64(count))

	return nil
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
