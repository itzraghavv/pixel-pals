# Pixel Pals — Product Requirement Docs

Pixel Pals is a learning/portfolio rebuild of a 2D virtual-office platform in the spirit of [Gather](https://www.gather.town/): people walk avatars around a tile-based map, see each other move in real time, and start talking (text, then audio/video) when they get close.

The first version of the backend was written in TypeScript (Express + `ws` + Prisma). This repo is the rebuild: **Go backend, React/Next.js frontend, and production-style infrastructure**, documented first and built second.

## Documents

| # | Doc | What it covers |
|---|-----|----------------|
| 00 | [Product Overview](./00-product-overview.md) | Vision, personas, scope, non-goals, success metrics |
| 01 | [Backend (HTTP API) PRD](./01-backend-api-prd.md) | Go REST service: auth, users, avatars, elements, maps, spaces |
| 02 | [Realtime Service PRD](./02-realtime-prd.md) | WebSocket protocol, rooms, movement, proximity, horizontal scaling |
| 03 | [Frontend PRD](./03-frontend-prd.md) | Next.js app, game canvas, lobby, space editor, admin console |
| 04 | [Audio/Video & Chat PRD](./04-av-chat-prd.md) | Proximity A/V via WebRTC SFU, text chat, private areas |
| 05 | [Infrastructure PRD](./05-infrastructure-prd.md) | Repo layout, Docker, CI/CD, deployment, observability, security |
| 06 | [Roadmap & Milestones](./06-roadmap.md) | Phased delivery plan with exit criteria |

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
| A/V | LiveKit (self-hosted SFU, written in Go) |
| Object storage | S3-compatible (MinIO locally) |
| Frontend | Next.js (App Router) + TypeScript, Phaser 3 for the world canvas |
| State / data | Zustand, TanStack Query |
| Styling | Tailwind CSS + shadcn/ui |
| Infra | Docker, Docker Compose, GitHub Actions, Caddy, Fly.io or a VPS |
| Observability | `slog` JSON logs, Prometheus, Grafana, OpenTelemetry traces |
