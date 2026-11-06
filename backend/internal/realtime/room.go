package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	closeJoin     = 4001
	closeAccess   = 4003
	closeMissing  = 4004
	closeSlow     = 4008
	closeReplaced = 4009
)

type message struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type delivery struct {
	message
	frame *websocket.PreparedMessage
}

type closeReason struct {
	code    int
	message string
}

type identity struct {
	UserID    string `json:"userId"`
	AvatarURL string `json:"avatarUrl"`
	Username  string `json:"username"`
}

type presence struct {
	identity
	point
	Direction string `json:"dir,omitempty"`
	ID        string `json:"id,omitempty"`
}

type client struct {
	identity
	out      chan delivery
	stopped  chan struct{}
	stopOnce sync.Once
	reason   closeReason
}

func newClient() *client {
	return &client{out: make(chan delivery, 256), stopped: make(chan struct{})}
}

func (c *client) stop(reason closeReason) {
	c.stopOnce.Do(func() {
		c.reason = reason
		close(c.stopped)
	})
}

type participant struct {
	client    *client
	position  point
	direction string
	proximity proximity
}

type roomCommand struct {
	run      func()
	finished chan struct{}
}

type room struct {
	commands        chan roomCommand
	done            chan struct{}
	players         map[string]participant
	grid            grid
	compat          bool
	allowOverlap    bool
	proximityRadius int
	proximityDirty  bool
	metrics         *metrics
}

func newRoom(compat bool) *room {
	return &room{
		commands: make(chan roomCommand), done: make(chan struct{}),
		players: make(map[string]participant), compat: compat, allowOverlap: true,
	}
}

func (r *room) run(ctx context.Context, idle time.Duration) {
	timer := time.NewTimer(idle)
	defer close(r.done)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			for _, player := range r.players {
				player.client.stop(closeReason{1012, "Service restart"})
			}
			return
		case <-timer.C:
			return
		case command := <-r.commands:
			wasEmpty := len(r.players) == 0
			command.run()
			for r.proximityDirty {
				r.proximityDirty = false
				r.updateProximity()
			}
			switch {
			case wasEmpty && len(r.players) > 0:
				timer.Stop()
			case !wasEmpty && len(r.players) == 0:
				timer.Reset(idle)
			}
			close(command.finished)
		}
	}
}

// Once a command is accepted, wait for its result even if the caller disconnects.
// This lets the caller clean up an accepted join before returning.
func (r *room) call(ctx context.Context, command func()) bool {
	finished := make(chan struct{})
	select {
	case <-ctx.Done():
		return false
	case <-r.done:
		return false
	case r.commands <- roomCommand{command, finished}:
		<-finished
		return true
	}
}

var errNoSpawn = errors.New("space has no free spawn tile")

func (r *room) join(c *client) error {
	position, ok := r.spawn(c.UserID)
	if !ok {
		return errNoSpawn
	}

	self := r.presence(participant{client: c, position: position, direction: "down"})
	users := make([]presence, 0, len(r.players))
	for id, player := range r.players {
		if id != c.UserID {
			users = append(users, r.presence(player))
		}
	}
	var spawn *point
	if r.compat {
		spawn = &position
	}
	c.out <- delivery{message: message{"space-joined", struct {
		Self  presence   `json:"self"`
		Users []presence `json:"users"`
		Spawn *point     `json:"spawn,omitempty"`
	}{self, users, spawn}}}

	if old, exists := r.players[c.UserID]; exists {
		old.client.stop(closeReason{closeReplaced, "Session replaced"})
	}
	r.players[c.UserID] = participant{client: c, position: position, direction: "down"}
	r.proximityDirty = true
	r.broadcast(c.UserID, message{"user-joined", self})
	return nil
}

func (r *room) spawn(except string) (point, bool) {
	occupied := make(map[point]bool)
	if !r.allowOverlap {
		for id, player := range r.players {
			if id != except {
				occupied[player.position] = true
			}
		}
	}
	return r.grid.spawn(func(p point) bool { return occupied[p] })
}

func (r *room) occupied(p point, except string) bool {
	if !r.allowOverlap {
		for id, player := range r.players {
			if id != except && player.position == p {
				return true
			}
		}
	}
	return false
}

func (r *room) presence(p participant) presence {
	result := presence{identity: p.client.identity, point: p.position, Direction: p.direction}
	if r.compat {
		result.ID = result.UserID
	}
	return result
}

func (r *room) move(c *client, target point) {
	player, exists := r.players[c.UserID]
	if !exists || player.client != c {
		return
	}
	dir := direction(player.position, target)
	if dir == "" || !r.grid.free(target) || r.occupied(target, c.UserID) {
		r.rejectMove(c)
		return
	}
	r.proximityDirty = r.proximityDirty || r.proximityEdgesChanged(c.UserID, player.position, target)
	player.position = target
	player.direction = dir
	r.players[c.UserID] = player
	r.broadcastMovement(player, c.UserID)
}

func (r *room) rejectMove(c *client) {
	if player, exists := r.players[c.UserID]; exists && player.client == c {
		if !r.send(c, message{"movement-rejected", player.position}) {
			r.leave(c)
		}
	}
}

func (r *room) broadcastMovement(player participant, except string) {
	r.broadcast(except, message{"movement", struct {
		UserID string `json:"userId"`
		point
		Direction string `json:"dir"`
	}{player.client.UserID, player.position, player.direction}})
}

func (r *room) leave(c *client) {
	if player, exists := r.players[c.UserID]; exists && player.client == c {
		delete(r.players, c.UserID)
		r.proximityDirty = true
		r.broadcast("", message{"user-left", map[string]string{"userId": c.UserID}})
	}
}

func (r *room) send(c *client, msg message) bool {
	return r.enqueue(c, delivery{message: msg})
}

func (r *room) enqueue(c *client, msg delivery) bool {
	select {
	case <-c.stopped:
		return false
	default:
	}
	select {
	case c.out <- msg:
		return true
	default:
		c.stop(closeReason{closeSlow, "Slow consumer"})
		return false
	}
}

func (r *room) broadcast(except string, msg message) {
	if r.metrics != nil {
		start := time.Now()
		defer func() { r.metrics.broadcast.Observe(time.Since(start).Seconds()) }()
	}
	pending := []message{msg}
	for len(pending) > 0 {
		current := pending[0]
		pending = pending[1:]
		wire, _ := json.Marshal(current)
		frame, _ := websocket.NewPreparedMessage(websocket.TextMessage, wire)
		out := delivery{message: current, frame: frame}
		for id, player := range r.players {
			if id == except || r.enqueue(player.client, out) {
				continue
			}
			delete(r.players, id)
			r.proximityDirty = true
			pending = append(pending, message{"user-left", map[string]string{"userId": id}})
		}
		except = ""
	}
}
