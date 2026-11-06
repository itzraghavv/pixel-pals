package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
	"github.com/itzraghavv/pixel-pals/backend/internal/auth"
	"github.com/itzraghavv/pixel-pals/backend/internal/service"
	"github.com/itzraghavv/pixel-pals/backend/internal/store/db"
)

const (
	joinTimeout  = 5 * time.Second
	pingInterval = 20 * time.Second
	pongTimeout  = 45 * time.Second
	writeTimeout = 5 * time.Second
	roomIdle     = 60 * time.Second
)

type Repository interface {
	GetSpace(context.Context, string) (db.Space, error)
	GetSpaceElements(context.Context, string) ([]db.GetSpaceElementsRow, error)
	GetSpaceAreaTiles(context.Context, string) ([]db.GetSpaceAreaTilesRow, error)
	IsMember(context.Context, db.IsMemberParams) (bool, error)
	UserByID(context.Context, string) (db.User, error)
	BulkMetadata(context.Context, []string) ([]db.BulkMetadataRow, error)
}

type Options struct {
	Origins         []string
	Compat          bool
	AllowOverlap    bool
	ProximityRadius int
}

type Server struct {
	repo     Repository
	tokens   auth.Tokens
	options  Options
	log      *slog.Logger
	upgrader websocket.Upgrader

	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	rooms       map[string]*room
	clients     map[*client]struct{}
	connections sync.WaitGroup
}

func NewServer(repo Repository, tokens auth.Tokens, options Options, log *slog.Logger) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		repo: repo, tokens: tokens, options: options, log: log,
		ctx: ctx, cancel: cancel, rooms: make(map[string]*room), clients: make(map[*client]struct{}),
	}
	s.upgrader.CheckOrigin = func(r *http.Request) bool {
		originsSent := r.Header.Values("Origin")
		if len(originsSent) == 0 {
			return true // CLI and service clients do not send an Origin.
		}
		if len(originsSent) != 1 {
			return false
		}
		for _, allowed := range options.Origins {
			if originsSent[0] == allowed {
				return true
			}
		}
		return false
	}
	return s
}

func (s *Server) Shutdown() {
	s.mu.Lock()
	for c := range s.clients {
		c.stop(closeReason{1012, "Service restart"})
	}
	s.cancel()
	s.mu.Unlock()
	s.connections.Wait()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c := newClient()
	s.mu.Lock()
	if s.ctx.Err() != nil {
		s.mu.Unlock()
		http.Error(w, "Service restarting", http.StatusServiceUnavailable)
		return
	}
	s.connections.Add(1)
	s.clients[c] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, c)
		s.mu.Unlock()
		s.connections.Done()
	}()

	conn, err := s.upgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer conn.Close()
		s.writePump(conn, c)
	}()
	defer func() {
		c.stop(closeReason{websocket.CloseNormalClosure, ""})
		<-done
	}()

	conn.SetReadLimit(4096)
	deadline := time.Now().Add(joinTimeout)
	_ = conn.SetReadDeadline(deadline)
	var first inbound
	for {
		var ok bool
		first, ok, err = readMessage(conn)
		if err != nil {
			c.stop(closeReason{closeJoin, "Join required within five seconds"})
			return
		}
		if ok {
			break
		}
	}
	var join struct {
		SpaceID string `json:"spaceId"`
		Token   string `json:"token"`
	}
	if first.Type != "join" || json.Unmarshal(first.Payload, &join) != nil || join.SpaceID == "" || len(join.SpaceID) > 128 {
		c.stop(closeReason{closeJoin, "First message must be join"})
		return
	}
	if ids, exists := req.URL.Query()["spaceId"]; exists && (len(ids) != 1 || ids[0] != join.SpaceID) {
		c.stop(closeReason{closeAccess, "Join must match the routing spaceId"})
		return
	}

	ctx, cancel := context.WithDeadline(s.ctx, deadline)
	r, reason := s.join(ctx, c, join.SpaceID, join.Token)
	cancel()
	if r == nil {
		c.stop(reason)
		return
	}
	defer r.call(s.ctx, func() { r.leave(c) })
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(pongTimeout)) })
	_ = conn.SetReadDeadline(time.Now().Add(pongTimeout))
	moves, chats := newBucket(20, 20), newBucket(5, 10)
	for {
		msg, ok, err := readMessage(conn)
		if err != nil {
			return
		}
		if !ok {
			continue
		}
		switch msg.Type {
		case "move":
			if !moves.allow(time.Now()) {
				continue
			}
			var target struct {
				X *int `json:"x"`
				Y *int `json:"y"`
			}
			if json.Unmarshal(msg.Payload, &target) == nil && target.X != nil && target.Y != nil {
				r.call(s.ctx, func() { r.move(c, point{*target.X, *target.Y}) })
			}
		case "chat":
			var chat struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(msg.Payload, &chat) == nil && strings.TrimSpace(chat.Text) != "" &&
				utf8.RuneCountInString(chat.Text) <= 500 && chats.allow(time.Now()) {
				r.call(s.ctx, func() { r.chat(c, chat.Text) })
			}
		case "ping":
			var ping struct {
				T json.RawMessage `json:"t"`
			}
			if json.Unmarshal(msg.Payload, &ping) == nil && ping.T != nil {
				r.call(s.ctx, func() {
					if !r.send(c, message{"pong", ping}) {
						r.leave(c)
					}
				})
			}
		}
	}
}

func (s *Server) join(ctx context.Context, c *client, spaceID, token string) (*room, closeReason) {
	claims, err := s.tokens.Verify(token)
	if err != nil {
		return nil, closeReason{closeAccess, "Invalid access token"}
	}
	space, err := service.AuthorizeSpace(ctx, s.repo, spaceID, claims.Subject)
	if err != nil {
		switch {
		case service.Missing(err):
			return nil, closeReason{closeMissing, "Space not found"}
		case errors.Is(err, service.ErrSpaceMembership):
			return nil, closeReason{closeAccess, "Space membership required"}
		default:
			return nil, s.joinError(ctx, err)
		}
	}
	user, err := s.repo.UserByID(ctx, claims.Subject)
	if service.Missing(err) {
		return nil, closeReason{closeAccess, "User not found"}
	}
	if err != nil {
		return nil, s.joinError(ctx, err)
	}
	c.identity = identity{UserID: user.ID, Username: user.Username}
	if user.AvatarID.Valid {
		metadata, err := s.repo.BulkMetadata(ctx, []string{user.ID})
		if err != nil {
			return nil, s.joinError(ctx, err)
		}
		if len(metadata) > 0 {
			c.AvatarURL = metadata[0].ImageUrl
		}
	}

	for ctx.Err() == nil {
		r := s.getRoom(spaceID)
		if r == nil {
			break
		}
		if !r.call(ctx, func() {
			if r.grid.blocked == nil {
				err = s.loadGrid(ctx, r, space)
				if err != nil {
					return
				}
			}
			if ctx.Err() == nil {
				err = r.join(c)
			} else {
				err = ctx.Err()
			}
		}) {
			continue
		}
		if errors.Is(err, errNoSpawn) {
			return nil, closeReason{1013, err.Error()}
		}
		if err != nil {
			return nil, s.joinError(ctx, err)
		}
		return r, closeReason{}
	}
	return nil, s.joinError(ctx, ctx.Err())
}

func (s *Server) joinError(ctx context.Context, err error) closeReason {
	if s.ctx.Err() != nil {
		return closeReason{1012, "Service restart"}
	}
	if ctx.Err() != nil {
		return closeReason{closeJoin, "Join timed out"}
	}
	s.log.Error("realtime join failed", "error", err)
	return closeReason{1011, "Could not join space"}
}

func (s *Server) loadGrid(ctx context.Context, r *room, space db.Space) error {
	elements, err := s.repo.GetSpaceElements(ctx, space.ID)
	if err != nil {
		return err
	}
	areas, err := s.repo.GetSpaceAreaTiles(ctx, space.ID)
	if err != nil {
		return err
	}
	g, err := newGrid(space, elements)
	if err != nil {
		return err
	}
	g.areas = make(map[point]string, len(areas))
	for _, area := range areas {
		p := point{int(area.X), int(area.Y)}
		if g.free(p) {
			g.areas[p] = area.AreaID
		}
	}
	r.grid = g
	return nil
}

func (s *Server) getRoom(spaceID string) *room {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return nil
	}
	if r := s.rooms[spaceID]; r != nil {
		select {
		case <-r.done:
		default:
			return r
		}
	}
	r := newRoom(s.options.Compat)
	r.allowOverlap = s.options.AllowOverlap
	r.proximityRadius = s.options.ProximityRadius
	s.rooms[spaceID] = r
	go func() {
		r.run(s.ctx, roomIdle)
		s.mu.Lock()
		if s.rooms[spaceID] == r {
			delete(s.rooms, spaceID)
		}
		s.mu.Unlock()
	}()
	return r
}

type inbound struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func readMessage(conn *websocket.Conn) (inbound, bool, error) {
	kind, data, err := conn.ReadMessage()
	var msg inbound
	ok := err == nil && kind == websocket.TextMessage && json.Unmarshal(data, &msg) == nil && msg.Type != ""
	return msg, ok, err
}

func (s *Server) writePump(conn *websocket.Conn, c *client) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	write := func(msg message) error {
		_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		return conn.WriteJSON(msg)
	}
	closeConnection := func() {
		reason := c.reason
		if reason.code == 1012 {
			_ = write(message{"error", map[string]string{"code": "server-restart", "message": reason.message}})
		} else if reason.code != websocket.CloseNormalClosure {
			_ = write(message{"error", struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{reason.code, reason.message}})
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(reason.code, reason.message), time.Now().Add(writeTimeout))
	}
	for {
		// Prioritize a close request over queued broadcasts.
		select {
		case <-c.stopped:
			closeConnection()
			return
		default:
		}
		select {
		case <-c.stopped:
			closeConnection()
			return
		case msg := <-c.out:
			if write(msg) != nil {
				return
			}
		case <-ticker.C:
			if conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout)) != nil {
				return
			}
		}
	}
}
