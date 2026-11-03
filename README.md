# Pixel Pals

A small realtime 2D world where position provides context: walk up to people and chat with whoever is nearby. This is an infrastructure and learning project built around a Go HTTP API, a planned stateful realtime service, and a Next.js client.

The HTTP API's P0/P1 features are implemented. The web app is currently a starter; realtime movement, chat, and a playable client remain upcoming milestones. See the [product requirements](apps/docs/PRD.md) and [roadmap](apps/docs/06-roadmap.md).

## Backend

Read the [backend runbook](backend/README.md) for setup, API behavior, seeding and verification. Requires Go 1.25.1+ and Docker Compose.

```powershell
$env:JWT_SECRET = [Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes(48))
docker compose up --build -d
```

The API runs on `http://localhost:8080`, with `/healthz`, `/readyz` and `/metrics`. Compose starts PostgreSQL, Redis, MinIO and migrations. Its credentials and exposed loopback ports are for local development. Use the runbook to seed demo accounts with a password you choose.

- [OpenAPI 3.1 contract](backend/api/openapi.yaml)
- [Generated frontend API types](apps/web/lib/api/schema.d.ts)
- [Backend CI](.github/workflows/backend.yml)

## Repository

- `backend/`: Go API, services, generated SQL, migrations and admin CLI.
- `apps/web/`: Next.js client starter and generated API declarations.
- `apps/docs/`: product requirements and documentation app starter.
- `packages/`: shared UI, ESLint and TypeScript configuration.

Frontend tooling requires Node 24+ and pnpm 11.25.0. Install with `pnpm install`; run the client starter with `pnpm --filter web dev`. The existing workspace scripts remain available in `package.json`.
