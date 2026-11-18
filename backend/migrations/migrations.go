// Package migrations embeds the SQL migration files so they ship inside the binary.
package migrations

import "embed"

// FS holds all *.sql files in this directory, applied in lexical order.
//
//go:embed *.sql
var FS embed.FS
