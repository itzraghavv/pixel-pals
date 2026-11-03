.PHONY: api test migrate seed generate infra
api:
	cd backend && go run ./cmd/api
test:
	cd backend && go test -race ./...
migrate:
	cd backend && go run ./cmd/palsctl migrate
seed:
	cd backend && go run ./cmd/palsctl seed
generate:
	cd backend && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 generate
infra:
	docker compose up -d postgres redis minio minio-init
