# Pixel Pals Backend

Go implementation of [Backend API PRD](../apps/docs/01-backend-api-prd.md), guided by the [product overview](../apps/docs/00-product-overview.md). The API owns persistent accounts, catalogue and spaces. The [realtime service](../apps/docs/02-realtime-prd.md) owns active rooms, movement, presence, proximity and live space-wide chat.

## Run locally

Requires Go 1.25.1+, Docker Compose, and optionally Make. Run the following from the repository root in PowerShell:

```powershell
$env:JWT_SECRET = [Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes(48))
docker compose up --build -d
```

This starts PostgreSQL 16, Redis 7, MinIO, the bucket initializer, migrations, the API on `http://localhost:8080` and realtime on `ws://localhost:8081/ws`. Local infrastructure ports bind to loopback. MinIO console is on `http://localhost:9001` (`minioadmin` / `minioadmin`). The Compose credentials are for local development. The playable web client is a later milestone.

To run the API directly instead, start dependencies with `docker compose up -d postgres redis minio minio-init`, set variables from [.env.example](.env.example), then:

```powershell
cd backend
$env:DATABASE_URL = 'postgres://pals:pals@localhost:5432/pals?sslmode=disable'
$env:REDIS_URL = 'redis://localhost:6379'
$env:APP_ENV = 'development'
$env:CORS_ORIGINS = 'http://localhost:3000'
$env:COMPAT_V0 = 'true'
go run ./cmd/palsctl migrate
$env:SEED_PASSWORD = 'choose-a-local-demo-password'
go run ./cmd/palsctl seed
go run ./cmd/api
```

Environment files are examples; Go does not automatically load them. Set `S3_*` variables to enable uploads when running directly. The same Make targets are available as `make migrate`, `make seed`, `make api`, `make realtime`, `make generate` and `make test`.

Seed creates `demo-admin`, `demo-user`, an avatar, a blocking element, and two templates. Re-running seed preserves existing data/passwords. Seed asset URLs are placeholders; replace them with hosted pixel art. To promote an existing user: `go run ./cmd/palsctl promote-admin <username>`, then sign in again for a new role claim. Admin sign-up is off by default and can only be enabled with both `APP_ENV=development` and `ALLOW_ADMIN_SIGNUP=true`.

## API and compatibility

The complete OpenAPI 3.1 contract is [api/openapi.yaml](api/openapi.yaml). It documents all request and response shapes. Regenerate frontend declarations from the root with:

```sh
pnpm dlx openapi-typescript@7.6.1 backend/api/openapi.yaml -o apps/web/lib/api/schema.d.ts
```

`COMPAT_V0=true` adds top-level `message` to structured errors, returns 403 for unknown maps when creating spaces and 400 for unknown spaces. `/api/v1/element` aliases `/api/v1/elements`. Successful legacy operations return 200. Invalid/missing access credentials return 401/403 respectively. Bounds validate the full element footprint, including exclusive right and bottom edges. Space reads require owner/member access in both modes.

Access JWTs expire after 15 minutes. Refresh credentials are random, hashed in PostgreSQL, valid for 30 days and atomically rotated. Logout revokes the refresh session; existing access JWTs expire normally. Cookies always use `HttpOnly; Secure; SameSite=Lax` and path `/api/v1/auth`. Use HTTPS for deployed clients; browser localhost exceptions vary. Credentialed cross-origin requests need `credentials: 'include'` and an exact `CORS_ORIGINS` entry. Refresh/logout reject a supplied Origin outside that list.

Argon2id uses 64 MiB, two iterations and one lane. The legacy verifier assumes Node `scrypt(password, saltString, 64)` defaults (`N=16384,r=8,p=1`) with `salt.hexDigest` storage; the old TypeScript implementation was not supplied for direct verification. UUIDv7 strings identify new records; database text IDs also accept existing CUIDs.

Sign-in limits are shared in Redis: 10 attempts/minute per socket-peer IP + case-insensitive username. Redis failures fail sign-in closed. Forwarded IP headers are intentionally not trusted; configure a trusted-proxy boundary before using forwarded addresses in production. Unit tests use an in-process limiter. Requests are capped at 1 MiB. Dimensions and footprints are limited to 1,000 tiles per axis.

Invites expire after seven days, are stored hashed and may be shared with multiple members. Acceptance is idempotent. Owner status remains authoritative on `spaces.creator_id`. Element changes and space deletion enqueue durable invalidations in the same transaction. A worker publishes `{"type":"space.updated","spaceId":"...","eventId":123}` on Redis channel `space.updated`, then acknowledges the outbox entry. Delivery is at least once; subscribers must reload idempotently. Pub/Sub only reaches online subscribers, so reconnecting rooms must load current state from PostgreSQL.

Presigned S3 PUT URLs expire after five minutes and bind both exact size (at most 2 MiB) and `image/png` / `image/webp` content type. Send the returned signed headers; browsers set `Content-Length` from the file automatically. Configure bucket public reads and CORS for your web origin in production. The API constrains declared media type, not the actual image bytes.

## Realtime

With the API and dependencies running, start the WebSocket service directly with `go run ./cmd/realtime` from `backend/`. It uses the same `DATABASE_URL`, `REDIS_URL`, `JWT_SECRET`, and `CORS_ORIGINS` as the API. `REALTIME_PORT` defaults to 8081, `ALLOW_TILE_OVERLAP` defaults to `true`, and `PROXIMITY_RADIUS` defaults to 3.

1. Sign in through the API and create or accept an invite to a space.
2. Open `ws://localhost:8081/ws?spaceId=<id>`. Send the API access token in the first message within five seconds.
   ```json
   {"type":"join","payload":{"spaceId":"<id>","token":"<access-token>"}}
   ```
3. Read your initial position from `space-joined.payload.self`. Send `{"type":"move","payload":{"x":1,"y":0}}` with coordinates one tile from your current position. Other clients get `movement`. Use `movement-rejected` to restore the server position. Browser WebSocket clients automatically answer the transport heartbeat.

In `COMPAT_V0=true`, join and presence payloads include the legacy `spawn` and `id` fields. A second socket for the same user and space replaces the first with close code 4009. Layout updates trigger `space-updated`, and the room relocates players whose tiles became blocked. Chat is delivered live to the room. Chat history, nearby chat, and A/V are tracked in PRD 04.

Private areas are tile tags scoped to a space. Tag tiles directly in PostgreSQL until the editor is available. Write the change and its outbox invalidation together, using the target space ID and in-bounds tile coordinates.

```sql
BEGIN;
INSERT INTO space_area_tiles (space_id, x, y, area_id)
VALUES ('space-id', 2, 2, 'quiet')
ON CONFLICT (space_id, x, y) DO UPDATE SET area_id = EXCLUDED.area_id;
INSERT INTO space_events (space_id) VALUES ('space-id');
COMMIT;
```

### Routing multiple nodes

Use a stable upstream pool and hash `spaceId` from the WebSocket URL. Realtime validates the query value against the join payload. Keep one authoritative room per space by draining or reconnecting affected sockets when changing the upstream pool. Redis carries layout invalidations between nodes. [Caddy load-balancing documentation](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy#load-balancing)

```caddyfile
handle /ws {
    @room query spaceId=*
    handle @room {
        reverse_proxy realtime-a:8081 realtime-b:8081 {
            lb_policy query spaceId
        }
    }
    handle {
        respond "spaceId required" 400
    }
}
```

Realtime `/metrics` exposes active connections, active room actors, message counters by direction, and a histogram for encoding and queuing broadcasts. Keep it on private routing. A graceful restart sends `error` with code `server-restart` and closes sockets with 1012 so clients can reconnect.

### Load checks

Run from `backend/` with `go test -tags load -count=1 -run TestRealtimeLoad -v ./internal/realtime`. The suite runs 50 bots in a single room, then 1,000 bots across 20 rooms and two realtime instances behind a test proxy that routes by space. Each bot moves four times per second for five seconds. The suite checks delivery to every peer and measures send-to-receive latency at one observer per room. Run the single-room case with two vCPUs to check the PRD's p95 target. The two instances run in one test process with a fixture repository and local sockets.

In the local Docker run, 50 bots delivered all 49,000 peer broadcasts at **1.1 ms p95** with a two-vCPU limit. The 1,000-bot run delivered all 980,000 peer broadcasts at **471 ms p95** on the shared eight-vCPU Docker VM.

## Checks and observability

```powershell
cd backend
go test ./...
go vet ./...
go test -race ./...
# Docker-backed PostgreSQL container, created and removed by testcontainers:
go test -tags integration ./internal/http '-coverpkg=./internal/http,./internal/service' -coverprofile=coverage.out
# Also verify Redis delivery/shared rate limits and real S3 uploads:
$env:TEST_REDIS_URL = 'redis://localhost:6379'
$env:TEST_S3_ENDPOINT = 'http://localhost:9000'
go test -tags integration ./...
```

`TEST_DATABASE_URL` can point the HTTP suite at an existing PostgreSQL instance; tests create and remove an isolated schema, leaving application tables intact. The original v0 `tests/index.test.js` is absent from this repository. The new Go HTTP contract suite covers the supplied PRD, but cannot prove parity with that missing suite. CI runs unit, race, PostgreSQL testcontainer, Redis and MinIO checks, verifies sqlc/type generation drift and builds the container.

`/healthz` reports process liveness; `/readyz` checks DB and Redis with a two-second deadline. `/metrics` exports HTTP requests/durations by route template, DB pool gauges, and Go runtime metrics. JSON request logs include request ID, authenticated user ID, status and latency; request bodies and credentials are not logged. Deploy metrics behind private routing. SIGTERM stops accepting requests and drains for at most 15 seconds.

Implemented scope: API P0 and P1 requirements. P2 soft deletion, cursor pagination and client-generated thumbnails remain deferred. Realtime P0/P1 features are implemented. Reconnect position persistence, binary encoding and emotes are deferred. The original v0 WebSocket suite is absent, so compatibility is covered by the Go socket tests. Production deployment and the remaining infrastructure milestones are tracked separately.
