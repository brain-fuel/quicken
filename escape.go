package quicken

import "html/template"

// Escape returns s with HTML metacharacters escaped, for interpolating
// untrusted data into a dynamic Tree slot. Tree emits slot content raw
// (regions own their markup), so wrap any user-derived value with Escape.
func Escape(s string) string { return template.HTMLEscapeString(s) }
