package realtime

import (
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestMoveBudget(t *testing.T) {
	b := newBucket(20, 20)
	at := b.at
	for range 20 {
		if !b.allow(at) {
			t.Fatal("burst exhausted early")
		}
	}
	if b.allow(at.Add(49*time.Millisecond)) || !b.allow(at.Add(50*time.Millisecond)) {
		t.Fatal("expected exactly one token after 50ms")
	}
}

func TestChatAndDuplicateSession(t *testing.T) {
	_, url, tokens := startFixture(t)
	token := issue(t, tokens, "a")
	old, _ := joinSocket(t, url, token)
	current, _ := joinSocket(t, url, token)
	read(t, old, "error")
	if _, _, err := old.ReadMessage(); !websocket.IsCloseError(err, closeReplaced) {
		t.Fatalf("duplicate close: %v", err)
	}
	b, _ := joinSocket(t, url, issue(t, tokens, "b"))
	read(t, current, "user-joined")
	send(t, current, "chat", map[string]string{"text": strings.Repeat("x", 501)})
	send(t, current, "chat", map[string]string{"text": "hello"})
	for _, c := range []*websocket.Conn{current, b} {
		if msg := string(read(t, c, "chat")); !strings.Contains(msg, "\"text\":\"hello\"") || !strings.Contains(msg, "\"userId\":\"a\"") {
			t.Fatalf("chat payload: %s", msg)
		}
	}
}
