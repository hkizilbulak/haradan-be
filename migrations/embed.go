package migrations

import "embed"

// FS embeds phase-one SQL migration files.
//
//go:embed *.sql
var FS embed.FS
