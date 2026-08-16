// Package migrations embeds the golang-migrate SQL files so that the server
// binary can run migrations without shipping the .sql files separately.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
