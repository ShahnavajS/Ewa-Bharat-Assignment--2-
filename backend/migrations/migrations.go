package migrations

import "embed"

// FS embeds all SQL migration files in this directory into the Go binary.
//
//go:embed *.sql
var FS embed.FS
