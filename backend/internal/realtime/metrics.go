package realtime

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type metrics struct {
	connections prometheus.Gauge
	rooms       prometheus.Gauge
	in, out     prometheus.Counter
	broadcast   prometheus.Histogram
	handler     http.Handler
}

func newMetrics() *metrics {
	registry := prometheus.NewRegistry()
	messages := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "realtime_messages_total", Help: "Application messages by direction.",
	}, []string{"direction"})
	m := &metrics{
		connections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "realtime_active_connections", Help: "Open WebSocket connections.",
		}),
		rooms: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "realtime_active_rooms", Help: "Live room actors, including rooms awaiting idle eviction.",
		}),
		in:  messages.WithLabelValues("in"),
		out: messages.WithLabelValues("out"),
		broadcast: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "realtime_broadcast_seconds", Help: "Time spent encoding and queuing a room broadcast.",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
		}),
		handler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	}
	registry.MustRegister(m.connections, m.rooms, messages, m.broadcast, collectors.NewGoCollector())
	return m
}

func (s *Server) Metrics() http.Handler { return s.metrics.handler }
