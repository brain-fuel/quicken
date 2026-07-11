package quicken

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"goforge.dev/cadence"
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

// strategyFor resolves the concrete cadence.Strategy for a region. A nil
// Policy falls back to the kind-inferred default: a LiveRegion gets Live, a
// plain Region gets Deferred{Server,OnLoad}. A non-nil Policy is consulted
// via StrategyFor. Deferred{Client} is rejected: it requires the TEA
// interpreter, which SP2 does not yet provide.
func strategyFor(p *Page, pol cadence.Policy, id string, ctx cadence.RequestContext) (cadence.Strategy, error) {
	var s cadence.Strategy
	if pol == nil {
		if _, live := p.live[id]; live {
			s = cadence.Strategy{Kind: cadence.Live}
		} else {
			s = cadence.Strategy{Kind: cadence.Deferred, Where: cadence.Server, On: cadence.OnLoad}
		}
	} else {
		s = pol.StrategyFor(id, ctx, nil)
	}
	if s.Kind == cadence.Deferred && s.Where == cadence.Client {
		return s, errors.New("quicken: Deferred{Client} strategy for region " + id + " requires the TEA interpreter (SP3), not available in SP2")
	}
	return s, nil
}

// tagOf maps a resolved cadence.Strategy to the fillTag the streamed floor
// uses to label a region's fill. It is a controlled switch: the sole
// resolver feeding renderFloor's tags, and the only place fillTag's enum
// strings are produced.
func tagOf(s cadence.Strategy) fillTag {
	switch s.Kind {
	case cadence.Eager:
		return fillTag{Strategy: "eager", Trigger: "onload"}
	case cadence.Live:
		return fillTag{Strategy: "live", Trigger: ""}
	default: // Deferred{Server, *}
		trig := "onload"
		switch s.On {
		case cadence.OnVisible:
			trig = "onvisible"
		case cadence.OnHover:
			trig = "onhover"
		}
		return fillTag{Strategy: "deferred", Trigger: trig}
	}
}

// serveComposite mounts a page on mux at path. Each request resolves every
// region's cadence.Strategy from pol (nil pol = kind-inferred default) and
// streams the universal floor: shell + skeletons first, then every region's
// full content tagged with its strategy so the client shim reveals it
// accordingly. With scripting off the floor is the page. (Exposed as the
// public Serve at the Task 6 cutover; live wiring added in Task 4.)
func serveComposite(mux *http.ServeMux, path string, p *Page, pol cadence.Policy) {
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := cadence.RequestContext{Path: r.URL.Path}
		resolve := func(id string) fillTag {
			s, err := strategyFor(p, pol, id, ctx)
			if err != nil {
				// SP2-unsupported (Deferred{Client}): degrade to eager so the
				// floor still shows full content. True client-compute is SP3.
				return fillTag{Strategy: "eager", Trigger: "onload"}
			}
			return tagOf(s)
		}
		_ = renderFloor(w, r, p, resolve)
	}))
}
