# 05 — Infrastructure PRD

**Status:** Draft

## 1. Goals

- One command to run everything locally.
- Reproducible builds, automated tests and deploys on every push.
- A small but realistic production setup: TLS, secrets, migrations, monitoring, backups.
- Keep cost near zero for a portfolio deployment.

## 2. Repository layout (monorepo)

```
pixel-pals/
├── backend/                 # single Go module
│   ├── cmd/
│   │   ├── api/             # REST service
│   │   ├── realtime/        # WebSocket service
│   │   └── palsctl/         # admin CLI: seed, promote-admin, migrate
│   ├── internal/            # shared packages (auth, store, config, ...)
│   ├── migrations/
│   ├── api/openapi.yaml
│   └── sqlc.yaml
├── web/                     # Next.js app
├── infra/
│   ├── docker/              # Dockerfiles
│   ├── compose/             # docker-compose.yml (+ .override for dev)
│   ├── caddy/Caddyfile
│   ├── livekit/livekit.yaml
│   ├── grafana/             # dashboards as JSON
│   └── k8s/                 # P2: manifests / Helm chart
├── tests/
│   ├── e2e/                 # Playwright
│   ├── contract/            # ported v0 Jest suite
│   └── load/                # k6 scripts + Go bot swarm
├── docs/                    # these PRDs + ADRs
├── Makefile
└── .github/workflows/
```

Both Go services share one module so `auth`, `store` and `config` are reused; they are built as two separate binaries and deployed independently.

## 3. Local development

| ID | Pri | Requirement |
|----|-----|-------------|
| INF-DEV-01 | P0 | `docker compose up` starts Postgres, Redis, MinIO, api, realtime, web, Caddy. LiveKit sits behind an opt-in `av` compose profile for the stretch goal. |
| INF-DEV-02 | P0 | `make dev` runs Go services with live reload (`air`) and Next.js dev server against compose infra. |
| INF-DEV-03 | P0 | `make migrate`, `make seed` (demo avatars, elements, two map templates, demo users). |
| INF-DEV-04 | P0 | `.env.example` documents every variable; no secrets committed. |
| INF-DEV-05 | P1 | Caddy routes `/api/*` → api, `/ws` → realtime, `/` → web on `localhost` so cookies/CORS match prod. |

## 4. Containers

| ID | Pri | Requirement |
|----|-----|-------------|
| INF-IMG-01 | P0 | Multi-stage Dockerfiles; Go images built `CGO_ENABLED=0` on `distroless/static` (< 30 MB). |
| INF-IMG-02 | P0 | Next.js built with `output: "standalone"`; non-root user. |
| INF-IMG-03 | P1 | Images tagged with git SHA and pushed to GHCR. |
| INF-IMG-04 | P1 | Trivy scan in CI; fail on critical CVEs. |

## 5. CI/CD (GitHub Actions)

| ID | Pri | Requirement |
|----|-----|-------------|
| INF-CI-01 | P0 | On PR: `go vet`, `golangci-lint`, `go test -race ./...` with Postgres/Redis service containers; `pnpm lint typecheck test` for web. |
| INF-CI-02 | P0 | Contract tests: spin up compose, run ported v0 Jest suite against Go services. |
| INF-CI-03 | P1 | Playwright E2E on PR against compose stack. |
| INF-CI-04 | P1 | On merge to `main`: build + push images, run migrations as a one-off job, deploy. |
| INF-CI-05 | P1 | Path filters so backend-only changes don't rebuild web and vice versa. |
| INF-CI-06 | P2 | Preview environments per PR. |

## 6. Production deployment

Target: **Fly.io** (simple, supports WebSockets and UDP for LiveKit) — or a single small VPS (Hetzner) running Docker Compose + Caddy. Both documented; pick one in an ADR.

| ID | Pri | Requirement |
|----|-----|-------------|
| INF-PROD-01 | P0 | HTTPS everywhere (Caddy auto-TLS or platform TLS); `wss://` for realtime. |
| INF-PROD-02 | P0 | Managed or volume-backed Postgres with daily backups (retain 7 days); restore tested once. |
| INF-PROD-03 | P0 | Secrets from platform secret store, never in images. |
| INF-PROD-04 | P1 | Two realtime instances behind sticky routing on `spaceId` to prove horizontal scaling. |
| INF-PROD-05 | P1 | Object storage on Cloudflare R2 / S3 with public read for assets, CORS for presigned uploads. |
| INF-PROD-06 | Stretch | LiveKit self-hosted with UDP port range open, or LiveKit Cloud free tier. |
| INF-PROD-07 | P2 | Kubernetes path: Helm chart, HPA on api, realtime as StatefulSet with session affinity. |

## 7. Observability

| ID | Pri | Requirement |
|----|-----|-------------|
| INF-OBS-01 | P0 | JSON logs via `slog` with `request_id`, `user_id`, `space_id`. |
| INF-OBS-02 | P1 | Prometheus scraping api + realtime (+ LiveKit if built); Grafana dashboards: HTTP RED metrics, WS connections/rooms, message rates, DB pool, Go runtime. |
| INF-OBS-03 | P1 | OpenTelemetry tracing from web → api → Postgres; exported to Grafana Tempo or Jaeger locally. |
| INF-OBS-04 | P2 | Alerts: error rate > 2%, p95 > 300 ms, realtime node down. |

## 8. Security

| ID | Pri | Requirement |
|----|-----|-------------|
| INF-SEC-01 | P0 | CORS allowlist; WS `Origin` check. |
| INF-SEC-02 | P0 | Request body size limits; upload type/size limits. |
| INF-SEC-03 | P0 | Parameterised SQL only (sqlc). |
| INF-SEC-04 | P1 | Security headers via Caddy (CSP, HSTS, X-Content-Type-Options). |
| INF-SEC-05 | P1 | Dependabot / Renovate for Go and npm deps. |

## 9. Architecture decision records

Record key choices in `docs/adr/NNNN-title.md` (context → decision → consequences). Initial ADRs:

1. Go monorepo with two binaries.
2. sqlc + pgx over an ORM.
3. Sticky routing for realtime scaling.
4. LiveKit for A/V (if the stretch goal is built).
5. Fly.io vs VPS hosting.
