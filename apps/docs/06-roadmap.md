# 06 — Roadmap & Milestones

Each milestone ends with something demoable. Don't start the next until the exit criteria pass. Infrastructure comes **before** product features: a minimal world that scales, deploys and is observable beats a feature-rich one that only runs on localhost.

## M0 — Foundations (week 1)

- Monorepo skeleton (`apps/web`, `apps/docs`, Go services), Makefile.
- Docker Compose: Postgres, Redis, MinIO.
- goose migrations for the full schema; sqlc configured; `palsctl seed`.
- CI: lint + test jobs for backend and web.

**Exit:** `docker compose up && make migrate seed` works on a clean machine; CI green.

## M1 — API parity with v0 (weeks 2–3)

- Auth (API-AUTH-01…03, 05, 08), user/avatars, catalogue, spaces (all P0 in [01](./01-backend-api-prd.md)).
- Port v0 Jest suite into `tests/contract`; run in CI with `COMPAT_V0=true`.

**Exit:** v0 HTTP tests pass against the Go API.

## M2 — Realtime core (weeks 3–4)

- Realtime P0s (RT-01…08): auth, collision grid, free-tile spawn, authoritative moves, broadcasts, heartbeats.
- Room-per-goroutine design with unit tests; `-race` clean.

**Exit:** v0 WS tests pass; two `wscat` clients see each other move.

## M3 — Minimal playable client (weeks 5–6)

- Auth pages, avatar picker, dashboard, new-space dialog.
- World canvas with prediction, interpolation, camera, participants list, reconnect.

**Exit:** Two browsers can sign up, enter a space and walk around together.

## M4 — Scale, observe, ship (weeks 7–9) ⭐ main infra milestone

- Production Dockerfiles, GHCR, deploy pipeline, TLS, backups (INF-IMG, INF-CI, INF-PROD in [05](./05-infrastructure-prd.md)).
- Two realtime nodes with sticky routing; Redis control events (RT-13, RT-16).
- Prometheus + Grafana dashboards; OpenTelemetry traces; structured logs.
- Load test with a Go bot swarm / k6: 1,000 bots across 20 spaces; results in `docs/load-test-results.md`.
- Chaos check: kill a realtime node mid-session and confirm clients recover in < 5 s.

**Exit:** Public demo URL; load-test report; Grafana screenshot; architecture diagram in README.

## M5 — Proximity & chat (weeks 10–11)

- RT-11 proximity groups, `proximity` events, private areas.
- Text chat (space-wide + nearby) with history.
- Refresh tokens, invites and access control (API-AUTH-04, API-SPACE-08).
- Editor P0s.

**Exit:** Chat works in both scopes; proximity ring and "nearby" badges update correctly.

## M6 — Unique features (weeks 12+)

- Pick 1–3 features from [07 — Feature Ideas](./07-feature-ideas.md).
- Write a short PRD for each (`08-…`, `09-…`) using the same template before building.

**Exit:** Each chosen feature is demoable and covered in the README.

## Stretch — Proximity audio/video

- LiveKit in compose; token endpoint; device check; proximity-scoped subscriptions; server-enforced subscriptions ([04](./04-av-chat-prd.md) §4.1).

**Exit:** AV acceptance criteria in [04](./04-av-chat-prd.md) met across two networks.

## Later / polish

- Walking animations, emotes, minimap, click-to-move.
- Guest links, OAuth.
- Kubernetes/Helm path.
- Write-up blog post: "Building a realtime multiplayer world in Go".

## Portfolio checklist

- [ ] README with demo GIF, architecture diagram, tech stack, "how to run".
- [ ] Live demo link with seeded demo accounts.
- [ ] ADRs explaining key decisions.
- [ ] Load-test results and Grafana screenshot.
- [ ] Clean commit history and PRs referencing requirement IDs.
