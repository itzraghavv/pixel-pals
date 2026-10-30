# 01 — Backend (HTTP API) PRD

**Status:** Draft · **Service:** `api` · **Language:** Go

## 1. Goals

- Rebuild the REST API in idiomatic Go with clear layering (handler → service → repository).
- Keep the v0 contract working under `/api/v1` so the existing black-box test suite can be pointed at the Go service.
- Fix known v0 issues (no token expiry, inconsistent status codes, unvalidated query params, missing ownership checks).
- Add asset uploads, refresh tokens, pagination and structured errors.

## 2. Non-goals

- GraphQL or gRPC public APIs.
- Realtime features (owned by the realtime service, see [02](./02-realtime-prd.md)).

## 3. Architecture

```
cmd/api/main.go           → config, wiring, graceful shutdown
internal/http/            → chi router, middleware, handlers, DTOs
internal/service/         → business rules (auth, space, catalogue)
internal/store/           → sqlc-generated queries + repository interfaces
internal/auth/            → password hashing, JWT issue/verify
internal/storage/         → S3/MinIO client for uploads
internal/platform/        → logging, metrics, tracing, config
migrations/               → goose SQL migrations
```

- **Router:** `chi` with middleware for request ID, real IP, recoverer, structured logging, CORS, rate limiting, auth.
- **DB:** PostgreSQL via `pgxpool`; queries written in SQL and generated with `sqlc`.
- **Validation:** request DTOs validated with `go-playground/validator`.
- **Errors:** every error response uses one shape:
  ```json
  { "error": { "code": "SPACE_NOT_FOUND", "message": "Space not found" } }
  ```
  In v1-compat mode the legacy `{ "message": "..." }` field is also included.

## 4. Data model

| Table | Key columns | Notes |
|-------|-------------|-------|
| `users` | `id`, `username` (unique, citext), `password_hash`, `role` (`admin`/`user`), `avatar_id` FK, `created_at` | |
| `refresh_tokens` | `id`, `user_id`, `token_hash`, `expires_at`, `revoked_at` | New in v1 |
| `avatars` | `id`, `name`, `image_url`, `created_at` | |
| `elements` | `id`, `width`, `height`, `static`, `image_url`, `created_at` | |
| `maps` | `id`, `name`, `width`, `height`, `thumbnail_url` | |
| `map_elements` | `id`, `map_id` FK, `element_id` FK, `x`, `y` (NOT NULL) | `x`,`y` were nullable in v0 |
| `spaces` | `id`, `name`, `width`, `height`, `thumbnail_url`, `creator_id` FK, `created_at` | |
| `space_elements` | `id`, `space_id` FK **ON DELETE CASCADE**, `element_id` FK, `x`, `y` | v0 lacked cascade, so space delete could fail |
| `space_members` *(P1)* | `space_id`, `user_id`, `role` (`owner`/`member`) | Enables invites and access control |
| `chat_messages` *(P1)* | `id`, `space_id`, `user_id`, `body`, `created_at` | Written by realtime service |

IDs: keep string IDs (CUID2 or UUIDv7) so the API surface does not change.

## 5. Requirements

### 5.1 Auth

| ID | Pri | Requirement |
|----|-----|-------------|
| API-AUTH-01 | P0 | `POST /api/v1/sign-up` with `{username, password, type}` creates a user. `type` ∈ `user`, `admin`. Returns `{userId}`. Duplicate username → `400`. |
| API-AUTH-02 | P0 | `POST /api/v1/sign-in` returns `{token}` (access JWT). Wrong credentials → `403`. |
| API-AUTH-03 | P0 | Access JWT claims: `sub` (user id), `role`, `iat`, `exp` (15 min). Signed HS256 with `JWT_SECRET`. Legacy `userId` claim also emitted for compatibility. |
| API-AUTH-04 | P1 | Refresh token (opaque, 30 days, stored hashed) returned as `HttpOnly; Secure; SameSite=Lax` cookie. `POST /api/v1/auth/refresh` rotates it; `POST /api/v1/auth/logout` revokes it. |
| API-AUTH-05 | P0 | Passwords hashed with argon2id (new users). Verifier also accepts v0 `salt.scryptHash` format and re-hashes on successful login. |
| API-AUTH-06 | P1 | Admin sign-up via `type: "admin"` is disabled outside dev (`ALLOW_ADMIN_SIGNUP=false`); admins are seeded or promoted by CLI. |
| API-AUTH-07 | P1 | Rate limit sign-in: 10 attempts / minute / IP + username. |
| API-AUTH-08 | P0 | `RequireUser` middleware: missing/invalid token → `403`/`401`. `RequireAdmin`: non-admin → `403`. |

### 5.2 User & avatars

| ID | Pri | Requirement |
|----|-----|-------------|
| API-USER-01 | P0 | `GET /api/v1/avatars` lists all avatars. |
| API-USER-02 | P0 | `POST /api/v1/user/metadata` `{avatarId}` sets the caller's avatar. Unknown avatar → `400`. |
| API-USER-03 | P0 | `GET /api/v1/user/metadata/bulk?ids=[a,b,c]` returns `{avatars:[{userId, imageUrl}]}`. Max 100 ids; malformed list → `400`. |
| API-USER-04 | P1 | `GET /api/v1/me` returns id, username, role, avatar. |

### 5.3 Catalogue (admin)

| ID | Pri | Requirement |
|----|-----|-------------|
| API-CAT-01 | P0 | `GET /api/v1/elements` lists elements (`id, imageUrl, width, height, static`). Fixes v0 `widthL` typo; v1-compat route `/api/v1/element` kept. |
| API-CAT-02 | P0 | `POST /api/v1/admin/element` creates an element. |
| API-CAT-03 | P0 | `PUT /api/v1/admin/element/{id}` updates `imageUrl`. Unknown id → `404`. |
| API-CAT-04 | P0 | `POST /api/v1/admin/avatar` creates an avatar. |
| API-CAT-05 | P0 | `POST /api/v1/admin/map` with `{name, thumbnail, dimensions:"WxH", defaultElements:[{elementId,x,y}]}` creates a map in one transaction. Elements outside bounds → `400`. |
| API-CAT-06 | P1 | `GET /api/v1/maps` lists map templates for the "new space" picker. |
| API-CAT-07 | P1 | `POST /api/v1/admin/uploads` returns a presigned S3 PUT URL (`image/png`, `image/webp`, ≤ 2 MB). Returned public URL is then used as `imageUrl`. |
| API-CAT-08 | P2 | Soft-delete for elements/avatars that are still referenced. |

### 5.4 Spaces

| ID | Pri | Requirement |
|----|-----|-------------|
| API-SPACE-01 | P0 | `POST /api/v1/space` `{name, dimensions, mapId?}`. With `mapId`, copy map size and elements in a single transaction. Unknown map → `400` (v0 returned `403`; compat mode keeps it). |
| API-SPACE-02 | P0 | `GET /api/v1/space/all` lists the caller's spaces (`id, name, thumbnail, dimensions`). |
| API-SPACE-03 | P0 | `GET /api/v1/space/{id}` returns dimensions and placed elements with nested element details. Unknown → `400` (compat) / `404` (v2). |
| API-SPACE-04 | P0 | `DELETE /api/v1/space/{id}` — owner only. Cascades to `space_elements`. |
| API-SPACE-05 | P0 | `POST /api/v1/space/element` `{spaceId, elementId, x, y}` — owner only; position must satisfy `0 ≤ x < width`, `0 ≤ y < height` (v0 allowed `x == width`). |
| API-SPACE-06 | P0 | `DELETE /api/v1/space/element` `{id}` — owner only. |
| API-SPACE-07 | P1 | After any element change, publish `space.updated` on Redis so live rooms reload their collision map. |
| API-SPACE-08 | P1 | Invites: `POST /api/v1/space/{id}/invites` returns a shareable token; `POST /api/v1/invites/{token}/accept` adds a member. Only owner/members can `GET` or join a space. |
| API-SPACE-09 | P2 | Space thumbnail auto-generated by the client editor and uploaded. |

### 5.5 Cross-cutting

| ID | Pri | Requirement |
|----|-----|-------------|
| API-X-01 | P0 | Config from env: `HTTP_PORT`, `DATABASE_URL`, `JWT_SECRET`, `REDIS_URL`, `S3_*`, `CORS_ORIGINS`, `APP_ENV`. Fails fast on missing required values. |
| API-X-02 | P0 | `GET /healthz` (liveness) and `GET /readyz` (DB + Redis reachable). |
| API-X-03 | P0 | Structured JSON logs with request id, user id, latency, status. |
| API-X-04 | P1 | Prometheus metrics at `/metrics`: request count/latency by route, DB pool stats. |
| API-X-05 | P1 | OpenAPI 3.1 spec checked in at `backend/api/openapi.yaml`; frontend types generated from it. |
| API-X-06 | P1 | Graceful shutdown on SIGTERM: stop accepting, drain in-flight requests (≤ 15 s). |
| API-X-07 | P2 | Cursor pagination on list endpoints (`?limit=&cursor=`). |

## 6. Acceptance criteria

- The original Jest black-box suite (`tests/index.test.js`) passes against the Go API with `COMPAT_V0=true`.
- Go unit tests for services and table-driven handler tests using `httptest`.
- Integration tests run against a real Postgres via `testcontainers-go` in CI.
- `sqlc generate` and `goose up` run cleanly from a fresh database.

## 7. Changes from v0

- Token expiry + refresh tokens (v0 tokens never expired).
- Cascade delete on spaces; bounds check is now exclusive.
- Ownership checked before bounds check (v0 dereferenced a possibly-null space).
- `JWT_SECRET` moved out of source code.
- Consistent error shape and `404` for missing resources (behind compat flag).
- Bulk metadata ids validated and capped.

## 8. Open questions

- Keep CUIDs or switch to UUIDv7? (Leaning UUIDv7 with compat for existing ids.)
- Should map templates be editable after spaces are created from them? (Proposed: no — spaces copy elements.)
