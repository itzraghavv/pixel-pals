# 00 — Product Overview

**Status:** Draft · **Owner:** @itzraghavv · **Last updated:** 2026-10-30

## 1. Problem

Remote teams lose the "ambient" part of being in an office: seeing who is around, bumping into someone, walking over to a desk for a quick chat. Video-call tools are scheduled and binary — you are either in a meeting or invisible.

Spatial platforms like Gather solve this with a 2D, game-like world where **position = context**: walk up to someone to talk, walk away to leave.

## 2. Why this project

This is a portfolio and learning project. The goal is to understand, end to end, how a realtime multiplayer product is built and run:

- A stateless REST API and a stateful realtime service, and why they are separate.
- Authoritative server-side movement and broadcasting in Go.
- Scaling WebSocket rooms across multiple instances with Redis.
- WebRTC through an SFU for proximity audio/video.
- A canvas-rendered client that stays in sync with the server.
- Containerised deployment, CI/CD and observability.

Success is measured by **working software and clear docs**, not users or revenue.

## 3. Personas

| Persona | Description | Main jobs |
|---------|-------------|-----------|
| **Member** | Regular user who joins spaces | Sign up, pick an avatar, join a space, walk around, talk to people nearby |
| **Space owner** | Member who creates spaces | Create a space from a map template or blank, place/remove furniture, invite people, delete space |
| **Admin** | Platform operator | Manage the global catalogue: avatars, elements (furniture/objects), map templates |
| **Guest** *(P2)* | Unauthenticated invitee | Join a space through an invite link with a display name |

## 4. Core user journeys

1. **Onboard:** Sign up → sign in → choose avatar → land on dashboard.
2. **Create a space:** Dashboard → "New space" → pick a map template (or blank + dimensions) → name it → space is created with the template's elements.
3. **Decorate:** Open the space editor → drag elements from the catalogue onto the grid → remove elements → save.
4. **Hang out:** Join a space → see own avatar and others → move with arrow keys/WASD → see others move in real time.
5. **Talk:** Walk near someone → audio/video tiles appear automatically → walk away → they disappear. Text chat in the space at all times.
6. **Administer:** Admin console → upload avatar/element images → build map templates.

## 5. Scope

### In scope (v1)

- Username/password auth with JWT access + refresh tokens.
- Avatars, elements, map templates (admin-managed catalogue).
- Spaces: create (blank or from map), list mine, view, delete; add/remove elements.
- Realtime: join space, authoritative grid movement, collision with static elements, join/leave/move broadcasts.
- Proximity detection and proximity A/V via LiveKit.
- Space-wide text chat.
- Web client: auth pages, dashboard, world canvas, space editor, admin console.
- Docker Compose for local dev; one-click deploy of the full stack; CI with tests.

### Non-goals (v1)

- Native mobile apps.
- OAuth / SSO providers (P2).
- Paid plans, billing, multi-tenant orgs.
- Tiled-map importer and full map-making tool on par with Gather's mapmaker.
- Screen sharing (P2), recordings, whiteboards, embedded websites/games.
- End-to-end encryption of media.

## 6. Success metrics (engineering)

| Metric | Target |
|--------|--------|
| Movement round-trip (client → server → other clients), same region | p95 < 100 ms |
| Concurrent users per space | ≥ 50 smooth on a single realtime node |
| Concurrent users total (load test, 2 realtime nodes) | ≥ 1,000 |
| Time from "walk up to someone" to hearing them | < 1.5 s |
| API p95 latency (non-upload) | < 50 ms |
| Backend test coverage of handlers/services | ≥ 70% |
| Cold start of full local stack (`docker compose up`) | < 2 min |

## 7. Glossary

- **Element** — a catalogue object with an image and a footprint (`width × height` tiles). `static` elements block movement (walls, desks); non-static ones are decorative (rugs).
- **Map** — an admin-created template: dimensions + default element placements.
- **Space** — a user-owned instance of a world. Created from a map (copying its elements) or blank.
- **Room** — the in-memory realtime representation of a space on a realtime node.
- **Tile** — one grid cell. All positions are integer tile coordinates.
- **Proximity group** — the set of users within *N* tiles of each other who should hear/see one another.

## 8. Risks

| Risk | Mitigation |
|------|-----------|
| WebRTC complexity balloons scope | Use LiveKit SFU instead of hand-rolled mesh; A/V is its own milestone |
| Realtime state lost on node restart | Positions are ephemeral by design; clients auto-rejoin with backoff |
| Asset hosting costs | MinIO locally, small S3/R2 bucket in prod, size limits on uploads |
| Scope creep from "replicate Gather" | Strict P0/P1/P2 tagging and milestone exit criteria |
