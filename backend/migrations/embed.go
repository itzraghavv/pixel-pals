package migrations

import "embed"

// Files includes the same goose migrations used by sqlc.
//
//go:embed *.sql
var Files embed.FS
