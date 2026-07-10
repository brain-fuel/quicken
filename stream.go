package quicken

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// StreamHTML flushes the shell with skeletons immediately, renders regions
// concurrently, and streams each region's real HTML as a fill block with an
// inline swap script as it becomes ready. With scripting disabled the fill
// blocks stay visible after the body content, so the page is still fully
// readable; the shim only relocates them into their slots.
type StreamHTML struct{}

// Deliver implements Transport.
func (StreamHTML) Deliver(w http.ResponseWriter, r *http.Request, p *Page) error {
	ctx := RenderContext{Ctx: r.Context(), R: r}
	frame := &Frame{page: p, ctx: ctx}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	doc := string(p.shell(frame))
	head, tail := splitBody(doc)

	if _, err := io.WriteString(w, head); err != nil {
		return err
	}
	flush(w)

	type result struct {
		id   string
		html string
	}
	results := make(chan result, len(p.order))
	for _, id := range p.order {
		id := id
		region := p.regions[id]
		go func() {
			results <- result{id: id, html: renderRegion(region, ctx)}
		}()
	}

	var done <-chan struct{}
	if ctx.Ctx != nil {
		done = ctx.Ctx.Done()
	}
	for range p.order {
		select {
		case res := <-results:
			fill := fmt.Sprintf(
				`<div data-q-fill="%s">%s</div><script>window.__quicken&&window.__quicken.swap(%s)</script>`,
				res.id, res.html, jsStringLiteral(res.id))
			if _, err := io.WriteString(w, fill); err != nil {
				return err
			}
			flush(w)
		case <-done:
			return ctx.Ctx.Err()
		}
	}

	_, err := io.WriteString(w, tail)
	return err
}

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
