package metrics

import (
	"context"
	"math"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const countTimeout = 2 * time.Second

type TaskCountReader interface {
	Count(ctx context.Context) (int64, error)
}

type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func New() *Metrics {
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
	}

	m.registry.MustRegister(
		m.requests,
		m.duration,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return m
}

func (m *Metrics) ObserveRequest(method, path, status string, d time.Duration) {
	m.requests.WithLabelValues(method, path, status).Inc()
	m.duration.WithLabelValues(method, path).Observe(d.Seconds())
}

func (m *Metrics) RegisterTaskGauge(reader TaskCountReader) error {
	return m.registry.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "tasks_count",
			Help: "Number of tasks that have not been soft-deleted.",
		},
		func() float64 {
			ctx, cancel := context.WithTimeout(context.Background(), countTimeout)
			defer cancel()

			count, err := reader.Count(ctx)
			if err != nil {
				return math.NaN()
			}

			return float64(count)
		},
	))
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
