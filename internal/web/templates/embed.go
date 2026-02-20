package templates

import "embed"

//go:embed pages/*.html partials/*.html base.html
var FS embed.FS
