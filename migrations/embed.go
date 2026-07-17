package migrations

import "embed"

// Files contains the immutable forward database migrations shipped with TeamPulse.
//
//go:embed *.up.sql
var Files embed.FS
