// Package migrations embeds the .sql files into the compiled binary,
// so the app (and its Docker image) needs no extra files at runtime.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
