# Pixel Pals — Product Requirement Docs

Pixel Pals is a realtime multiplayer 2D world: people walk avatars around a tile-based map, see each other move live, and chat with whoever is nearby.

It borrows the core idea of spatial platforms like [Gather](https://www.gather.town/) ("position = context") but is **not a clone**. The project has two goals:

1. **Infra first.** Build a production-style realtime system in Go: a stateless API, a stateful WebSocket service, horizontal scaling, CI/CD and observability. This is the main portfolio value.
2. **Its own identity later.** Once the infra works, add a few features Gather doesn't have. Candidates are collected in [07 — Feature Ideas](./07-feature-ideas.md) and haven't been chosen yet.

The Gather-style surface is kept deliberately small: spaces, movement, presence and proximity text chat. Proximity audio/video is a **stretch goal**.

The first version of the backend was written in TypeScript (Express + `ws` + Prisma). This repo is the rebuild: **Go backend, Next.js frontend, production-style infrastructure**, documented first and built second.

## Documents

| # | Doc | What it covers |
|---|-----|----------------|
| 00 | [Product Overview](./00-product-overview.md) | Vision, scope, non-goals, success metrics |
| 01 | [Backend (HTTP API) PRD](./01-backend-api-prd.md) | Go REST service: auth, users, avatars, elements, maps, spaces |
| 02 | [Realtime Service PRD](./02-realtime-prd.md) | WebSocket protocol, rooms, movement, proximity, horizontal scaling |
| 03 | [Frontend PRD](./03-frontend-prd.md) | Next.js app, game canvas, dashboard, editor, admin console |
| 04 | [Chat & A/V PRD](./04-av-chat-prd.md) | Text chat (core) and proximity audio/video via LiveKit (stretch) |
| 05 | [Infrastructure PRD](./05-infrastructure-prd.md) | Repo layout, Docker, CI/CD, deployment, observability, security |
| 06 | [Roadmap & Milestones](./06-roadmap.md) | Phased delivery plan with exit criteria |
| 07 | [Feature Ideas](./07-feature-ideas.md) | Backlog of unique features to choose from after the infra milestones |

## How to read these

- Each PRD has **Goals / Non-goals**, **Requirements** (tagged `P0` must-have, `P1` should-have, `P2` nice-to-have), **Acceptance criteria**, and **Open questions**.
- Requirement IDs (e.g. `API-AUTH-01`) are stable so issues and PRs can reference them.
- Where the rebuild intentionally differs from the original TypeScript backend, it is called out under **"Changes from v0"**.

## Tech stack at a glance

| Layer | Choice |
|-------|--------|
| Backend language | Go 1.23+ |
| HTTP router | `chi` |
| DB access | PostgreSQL 16 + `pgx` + `sqlc` |
| Migrations | `goose` |
| WebSocket | `coder/websocket` |
| Pub/Sub & presence | Redis 7 |
| Auth | JWT (access + refresh), `golang-jwt/jwt/v5`, argon2id/scrypt |
| Object storage | S3-compatible (MinIO locally) |
| Frontend | Next.js (App Router) + TypeScript, Phaser 3 for the world canvas |
| State / data | Zustand, TanStack Query |
| Styling | Tailwind CSS + shadcn/ui |
| Infra | Docker, Docker Compose, GitHub Actions, Caddy, Fly.io or a VPS |
| Observability | `slog` JSON logs, Prometheus, Grafana, OpenTelemetry traces |
| A/V *(stretch)* | LiveKit (self-hosted SFU, written in Go) |
