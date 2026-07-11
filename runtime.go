package quicken

import (
	"fmt"
	"io"
	"net/http"
)

// fillTag labels a streamed region fill so the client shim knows when to
// reveal it. Strategy is "eager", "deferred", or "live"; Trigger is
// "onload", "onvisible", "onhover", or "" (live).
type fillTag struct {
	Strategy string
	Trigger  string
}

// renderFloor writes the universal fallback floor: the shell head, then every
// region's full content streamed into the document tail as a tagged fill with
// an inline reveal script, then the shell tail. With scripting off the fills
// stay visible after the body, so the page is fully readable; the shim
// relocates them into their slots per strategy.
func renderFloor(w http.ResponseWriter, r *http.Request, p *Page, resolve func(id string) fillTag) error {
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
		go func() { results <- result{id: id, html: renderRegion(region, ctx)} }()
	}

	var done <-chan struct{}
	if ctx.Ctx != nil {
		done = ctx.Ctx.Done()
	}
	got := make(map[string]string, len(p.order))
	for range p.order {
		select {
		case res := <-results:
			got[res.id] = res.html
		case <-done:
			return ctx.Ctx.Err()
		}
	}
	for _, id := range p.order { // page order: deterministic output
		tag := resolve(id)
		fill := fmt.Sprintf(
			`<div data-q-fill="%s" data-q-strategy="%s" data-q-trigger="%s">%s</div><script>window.__quicken&&window.__quicken.reveal(%s)</script>`,
			id, tag.Strategy, tag.Trigger, got[id], jsStringLiteral(id))
		if _, err := io.WriteString(w, fill); err != nil {
			return err
		}
		flush(w)
	}
	_, err := io.WriteString(w, tail)
	return err
}
