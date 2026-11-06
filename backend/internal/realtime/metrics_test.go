package realtime

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsExposeRealtimeActivity(t *testing.T) {
	s, url, tokens := startFixture(t)
	a, spawn := joinSocket(t, url, issue(t, tokens, "a"))
	b, _ := joinSocket(t, url, issue(t, tokens, "b"))
	read(t, a, "user-joined")
	send(t, a, "move", point{1 - spawn.X, 0})
	read(t, b, "movement")

	recorder := httptest.NewRecorder()
	s.Metrics().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	for _, want := range []string{
		"realtime_active_connections 2",
		"realtime_active_rooms 1",
		"realtime_broadcast_seconds_count ",
		"realtime_messages_total{direction=\"in\"} 3",
		"realtime_messages_total{direction=\"out\"} ",
	} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Errorf("missing metric %q", want)
		}
	}
}
