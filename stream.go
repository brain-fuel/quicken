package quicken

import (
	"fmt"
	"net/http"
	"strconv"
)

// renderRegion runs a region's Render, turning a panic into an inline error
// card so one failing region cannot abort the whole page.
func renderRegion(region Region, ctx RenderContext) (html string) {
	defer func() {
		if rec := recover(); rec != nil {
			html = fmt.Sprintf(`<div data-q-error>region %q failed to render</div>`, region.ID())
		}
	}()
	return region.Render(ctx).HTML()
}

func flush(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// jsStringLiteral renders s as a JavaScript string literal for an inline
// script. This is safe only because region ids are restricted by validID to
// [A-Za-z0-9_-]+ on Add: strconv.Quote produces a valid JS string literal for
// that charset. strconv.Quote is not a general script-context escaper (it does
// not escape </script>, <, >, or U+2028/2029), so it must not be used on
// arbitrary strings here.
func jsStringLiteral(s string) string {
	return strconv.Quote(s)
}
