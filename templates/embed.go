package templates

import "embed"

// FS contains all HTML templates embedded into the binary.
//
//go:embed *.html
var FS embed.FS
