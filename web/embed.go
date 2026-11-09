// Package web embeds cakenify's templates and static assets so the built
// binary is self-contained (no relative-path lookups at runtime).
package web

import "embed"

//go:embed templates
var TemplatesFS embed.FS

//go:embed static
var StaticFS embed.FS
