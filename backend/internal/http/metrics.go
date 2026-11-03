package httpapi

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"strconv"
	"time"
)

type Metrics struct {
	Registry *prometheus.Registry
	Requests *prometheus.CounterVec
	Latency  *prometheus.HistogramVec
}

func NewMetrics(pool *pgxpool.Pool) *Metrics {
	m := &Metrics{Registry: prometheus.NewRegistry(), Requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_requests_total", Help: "HTTP requests by route and status."}, []string{"method", "route", "status"}), Latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP request duration.", Buckets: prometheus.DefBuckets}, []string{"method", "route"})}
	m.Registry.MustRegister(m.Requests, m.Latency, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	if pool != nil {
		m.Registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "db_pool_acquired_connections", Help: "Acquired PostgreSQL connections."}, func() float64 { return float64(pool.Stat().AcquiredConns()) }), prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "db_pool_idle_connections", Help: "Idle PostgreSQL connections."}, func() float64 { return float64(pool.Stat().IdleConns()) }), prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "db_pool_total_connections", Help: "Total PostgreSQL connections."}, func() float64 { return float64(pool.Stat().TotalConns()) }))
	}
	return m
}
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		status := ww.Status()
		if status == 0 {
			status = 200
		}
		m.Requests.WithLabelValues(r.Method, route, statusString(status)).Inc()
		m.Latency.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}
func statusString(status int) string { return strconv.Itoa(status) }
