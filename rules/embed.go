package rules

import "embed"

//go:embed builtin/*.yaml
var BuiltinFS embed.FS
