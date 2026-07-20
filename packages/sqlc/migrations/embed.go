// Package migrations exposes the canonical database migration tree to Go
// binaries that initialize the schema without shipping loose SQL files.
package migrations

import "embed"

// FS contains every canonical up and down migration.
//
//go:embed *.sql
var FS embed.FS
