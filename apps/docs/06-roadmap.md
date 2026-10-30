# 06 — Roadmap & Milestones

Each milestone ends with something demoable. Don't start the next until the exit criteria pass.

## M0 — Foundations (week 1)

- Monorepo skeleton, Go module, Next.js app, Makefile.
- Docker Compose: Postgres, Redis, MinIO.
- goose migrations for the full schema; sqlc configured.
- CI: lint + test jobs for backend and web.

**Exit:** `docker compose up && make migrate seed` works on a clean machine; CI green.

## M1 — API parity with v0 (weeks 2–3)

- Auth (API-AUTH-01…03, 05, 08), user/avatars, catalogue, spaces (all P0 in [01](./01-backend-api-prd.md)).
- Port v0 Jest suite into `tests/contract`; run in CI with `COMPAT_V0=true`.

**Exit:** v0 HTTP tests pass against the Go API.

## M2 — Realtime parity + correctness fixes (weeks 3–4)

- Realtime P0s (RT-01…08): auth, collision grid, free-tile spawn, authoritative moves, broadcasts, heartbeats.
- Room-per-goroutine design with unit tests; `-race` clean.

**Exit:** v0 WS tests pass; two `wscat` clients see each other move.

## M3 — Playable web client (weeks 5–7)

- Auth pages, avatar picker, dashboard, new-space dialog.
- World canvas with prediction, interpolation, camera, participants list, reconnect.
- Editor P0s.

**Exit:** Two browsers can sign up, create/enter a space, decorate it and walk around together. Record a demo GIF for the README.

## M4 — Proximity & chat (weeks 8–9)

- RT-11 proximity groups, `proximity` events, private areas.
- Text chat with history.
- Refresh tokens, invites and access control (API-AUTH-04, API-SPACE-08).

**Exit:** Chat works; proximity ring and "nearby" badges update correctly.

## M5 — Audio/Video (weeks 10–11)

- LiveKit in compose; token endpoint; device check; proximity-scoped subscriptions; mute states.
- Server-enforced subscriptions.

**Exit:** AV acceptance criteria in [04](./04-av-chat-prd.md) met across two networks.

## M6 — Production (weeks 12–13)

- Production Dockerfiles, GHCR, deploy pipeline, TLS, backups.
- Two realtime nodes with sticky routing.
- Prometheus + Grafana dashboards; OpenTelemetry traces.
- Load test: 1,000 bots; publish results in `docs/load-test-results.md`.

**Exit:** Public demo URL; load-test report; architecture diagram in README.

## M7 — Polish & stretch (ongoing)

- Walking animations, emotes, minimap, click-to-move.
- Screen sharing, guest links, OAuth.
- Kubernetes/Helm path.
- Write-up blog post: "Building a Gather-style virtual office in Go".

## Portfolio checklist

- [ ] README with demo GIF, architecture diagram, tech stack, "how to run".
- [ ] Live demo link with seeded demo accounts.
- [ ] ADRs explaining key decisions.
- [ ] Load-test results and Grafana screenshot.
- [ ] Clean commit history and PRs referencing requirement IDs.
