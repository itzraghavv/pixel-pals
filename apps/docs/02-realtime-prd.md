# 02 — Realtime Service PRD

**Status:** P0/P1 implemented locally · **Service:** `realtime` · **Language:** Go

## 1. Goals

- Authoritative, low-latency multiplayer movement inside a space.
- Presence: who is in the space, where they are, which way they face.
- Proximity detection that drives A/V (see [04](./04-av-chat-prd.md)).
- Run more than one realtime node behind a load balancer without users in the same space being split.

## 2. Non-goals

- Physics, free (non-grid) movement, or client-side authority.
- Persisting positions across sessions (P2 at most).

## 3. Architecture

```
Browser ──WS──▶ realtime node ──▶ Hub ──▶ Room (one goroutine per space)
                     │                        │
                     │                        ├─ collision grid (from DB)
                     │                        └─ users map[id]*Client
                     └──▶ Redis pub/sub  (cross-node fan-out, space.updated events)
```

- **Client:** one per WebSocket. Owns a read pump and a write pump goroutine and a buffered outbound channel. Slow consumers whose buffer fills are disconnected rather than blocking the room.
- **Room:** one goroutine per active space that serialises all state changes through a channel (no mutex juggling). Created on first join, torn down 60 s after last leave.
- **Hub:** `map[spaceID]*Room`, guarded by a mutex; also subscribes to Redis.
- **Scaling strategy (P1):** Two options documented, pick one in implementation:
  1. **Sticky routing** — LB hashes `spaceId` (query param) to a node so a room lives on one node. Simplest; Redis only used for control events.
  2. **Fan-out** — rooms on every node, moves published to `room:{spaceId}` channel. Needed only if single-space size exceeds one node.
  Default: **sticky routing**.

## 4. Protocol

Transport: WebSocket at `wss://<host>/ws`. Messages are JSON `{ "type": string, "payload": object }`. A versioned binary encoding (MessagePack) is P2.

### 4.1 Client → server

| Type | Payload | Notes |
|------|---------|-------|
| `join` | `{spaceId, token}` | Must be first message, within 5 s of connect, else close `4001`. |
| `move` | `{x, y, dir?}` | One tile in a cardinal direction from current position. |
| `chat` *(P1)* | `{text}` | ≤ 500 chars. |
| `emote` *(P2)* | `{emoji}` | |
| `ping` | `{t}` | App-level heartbeat; server replies `pong` with same `t`. |

### 4.2 Server → client

| Type | Payload | Notes |
|------|---------|-------|
| `space-joined` | `{self:{userId,x,y}, users:[{userId,x,y,avatarUrl,username}]}` | v0 sent only `{spawn, users:[{id}]}`; compat field `spawn` kept. |
| `user-joined` | `{userId, x, y, avatarUrl, username}` | To everyone except the joiner. |
| `movement` | `{userId, x, y, dir}` | v0 omitted `userId`; fixed. |
| `movement-rejected` | `{x, y}` | Authoritative position to snap back to. |
| `user-left` | `{userId}` | |
| `proximity` *(P1)* | `{groupId, peers:[userId]}` | Sent when the caller's proximity group changes. |
| `chat` *(P1)* | `{userId, text, ts}` | |
| `space-updated` *(P1)* | `{}` | Element layout changed; client refetches space. |
| `error` | `{code, message}` | Followed by close for fatal errors. |

### 4.3 Close codes

| Code | Meaning |
|------|---------|
| `4001` | Join timeout / not joined |
| `4003` | Auth failed or no access to space |
| `4004` | Space not found |
| `4008` | Rate limited / slow consumer |
| `4009` | Same user joined the space from another tab (older session kicked) |

## 5. Requirements

| ID | Pri | Requirement |
|----|-----|-------------|
| RT-01 | P0 | Verify JWT on `join` with the same secret/claims as the API; reject expired tokens. |
| RT-02 | P0 | Load space dimensions and **static elements** on room creation; build an occupancy grid. |
| RT-03 | P0 | Spawn on a random **free** tile (v0 could spawn inside walls or out of bounds). |
| RT-04 | P0 | Accept a move only if: exactly one tile in one axis, in bounds, tile not blocked by a static element. Otherwise send `movement-rejected`. |
| RT-05 | P0 | Broadcast `user-joined`, `movement`, `user-left` to all other clients in the room. |
| RT-06 | P0 | On socket close, remove user from room and broadcast `user-left` exactly once. |
| RT-07 | P0 | Ignore malformed JSON and unknown types without crashing the connection goroutine. |
| RT-08 | P0 | Server-side ping every 20 s; drop connections with no pong in 45 s. |
| RT-09 | P1 | Rate limit `move` to 20/s per client (token bucket); excess silently dropped. |
| RT-10 | P1 | Users may stand on the same tile (Gather allows overlap); configurable. |
| RT-11 | P1 | Proximity: after every move, recompute groups using Chebyshev distance ≤ `PROXIMITY_RADIUS` (default 3) with union-find; emit `proximity` only on change. |
| RT-12 | P1 | Private areas: tiles tagged with an `areaId` form closed groups regardless of distance. |
| RT-13 | P1 | Subscribe to `space.updated` on Redis; reload collision grid and notify clients. |
| RT-14 | P1 | Duplicate session for same user in same space: kick older with `4009`. |
| RT-15 | P1 | Metrics: active connections, active rooms, messages in/out per second, broadcast latency histogram. |
| RT-16 | P1 | Graceful shutdown: send `error{code:"server-restart"}` and close `1012` so clients reconnect elsewhere. |
| RT-17 | P2 | Persist last position per user/space in Redis for 10 min to respawn in place after reconnect. |

## 6. Acceptance criteria

- v0 websocket tests in `tests/index.test.js` pass with compat payloads enabled.
- Go tests: room unit tests drive the room via its channel with fake clients (no sockets); integration test with two real WS clients.
- Race detector (`go test -race`) clean.
- Load test (k6 or a Go bot swarm): 50 bots in one space moving at 4 tiles/s, p95 broadcast latency < 50 ms on a 2 vCPU node; 1,000 bots across 20 spaces on 2 nodes.

## 7. Open questions

- Tick-based batching (e.g. send state diffs every 50 ms) vs. immediate per-move broadcast. Start immediate; add batching if load tests demand it.
- Should chat history be persisted by realtime directly or via an API call? (Proposed: realtime writes asynchronously through a buffered channel.)
