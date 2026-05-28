package static

import "embed"

// FS contains the static assets bundled into the binary.
//
//go:embed preflight.css orchestrator.css fontawesome.css fa6/webfonts/fa-solid-900.woff2
var FS embed.FS
