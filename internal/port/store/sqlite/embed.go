package sqlite

import "embed"

// migrationsFS carries the embedded goose migrations applied at boot.
//
//go:embed migrations
var migrationsFS embed.FS
