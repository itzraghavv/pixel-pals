# 00 — Product Overview

**Status:** Draft · **Owner:** @itzraghavv · **Last updated:** 2026-10-30

**Implementation (2026-11-03):** The Go HTTP API foundation and P0/P1 backend
features are available; see the [backend runbook](../../backend/README.md).
Realtime, chat and the playable web client remain separate milestones. The
engineering success metrics below have not yet been established by load tests.

## 1. What Pixel Pals is

A small realtime 2D world where **position = context**: you see who's around, walk up to people and chat with whoever is nearby. The idea comes from spatial platforms like Gather, but Pixel Pals doesn't try to copy them feature for feature.

## 2. Why this project

This is a portfolio and learning project. **The infrastructure is the product.** The goal is to understand, end to end, how a realtime multiplayer system is built and run:

- A stateless REST API and a stateful realtime service, and why they are separate.
- Authoritative server-side movement and broadcasting in Go.
- Scaling WebSocket rooms across multiple instances with Redis.
- A canvas-rendered client that stays in sync with the server.
- Containerised deployment, CI/CD, load testing and observability.

The user-facing features stay minimal until that foundation works. After that, a few **unique features** give the project its own identity (see [07 — Feature Ideas](./07-feature-ideas.md); not chosen yet).

Success is measured by **working software, measurable performance and clear docs**, not users or revenue.

## 3. Personas

| Persona | Description | Main jobs |
|---------|-------------|-----------|
| **Member** | Regular user who joins spaces | Sign up, pick an avatar, join a space, walk around, chat with people nearby |
| **Space owner** | Member who creates spaces | Create a space from a map template or blank, place/remove furniture, invite people, delete space |
| **Admin** | Platform operator | Manage the global catalogue: avatars, elements (furniture/objects), map templates |

## 4. Core user journeys

1. **Onboard:** Sign up → sign in → choose avatar → land on dashboard.
2. **Create a space:** Dashboard → "New space" → pick a map template (or blank + dimensions) → name it.
3. **Hang out:** Join a space → see own avatar and others → move with arrow keys/WASD → see others move in real time.
4. **Chat:** Space-wide chat at all times; a "nearby" channel for whoever is within a few tiles.
5. **Decorate** *(P1)***:** Place and remove elements in your space.
6. **Administer:** Seed or upload avatars, elements and map templates.

## 5. Scope

### Core (v1)

- Username/password auth with JWT access + refresh tokens.
- Avatars, elements, map templates (admin-managed catalogue; seeding via CLI is enough at first).
- Spaces: create (blank or from map), list mine, view, delete; add/remove elements.
- Realtime: join space, authoritative grid movement, collision, join/leave/move broadcasts, proximity groups.
- Text chat: space-wide and proximity-scoped.
- Web client: auth pages, dashboard, world canvas. A simple editor and admin console come after.
- **Infra:** Docker Compose, CI with tests, multiple realtime nodes, metrics/tracing, load tests, public deployment.

### Later

- 1–3 unique features picked from [07 — Feature Ideas](./07-feature-ideas.md).
- Proximity audio/video via LiveKit (**stretch**, see [04](./04-av-chat-prd.md)).

### Non-goals

- Feature parity with Gather: mapmaker, embedded websites/games, whiteboards, recordings, screen sharing.
- Native mobile apps.
- OAuth / SSO providers (P2).
- Paid plans, billing, multi-tenant orgs.

## 6. Success metrics (engineering)

| Metric | Target |
|--------|--------|
| Movement round-trip (client → server → other clients), same region | p95 < 100 ms |
| Concurrent users per space | ≥ 50 smooth on a single realtime node |
| Concurrent users total (load test, 2 realtime nodes) | ≥ 1,000 |
| API p95 latency (non-upload) | < 50 ms |
| Realtime node restart: clients back in their space | < 5 s |
| Backend test coverage of handlers/services | ≥ 70% |
| Cold start of full local stack (`docker compose up`) | < 2 min |

## 7. Glossary

- **Element** — a catalogue object with an image and a footprint (`width × height` tiles). `static` elements block movement (walls, desks); non-static ones are decorative (rugs).
- **Map** — an admin-created template: dimensions + default element placements.
- **Space** — a user-owned instance of a world. Created from a map (copying its elements) or blank.
- **Room** — the in-memory realtime representation of a space on a realtime node.
- **Tile** — one grid cell. All positions are integer tile coordinates.
- **Proximity group** — the set of users within *N* tiles of each other; used for nearby chat (and A/V if built).

## 8. Risks

| Risk | Mitigation |
|------|-----------|
| Scope creep toward a full Gather clone | Core surface is fixed above; anything new goes into [07](./07-feature-ideas.md) first |
| WebRTC complexity | A/V is a stretch goal, built only after infra and unique features |
| Realtime state lost on node restart | Positions are ephemeral by design; clients auto-rejoin with backoff |
| Asset hosting costs | MinIO locally, small S3/R2 bucket in prod, size limits on uploads |
