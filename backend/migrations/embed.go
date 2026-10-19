package migrations

import "embed"

// FS holds every .sql migration so the binary carries its own schema.
//
//go:embed *.sql
var FS embed.FS
