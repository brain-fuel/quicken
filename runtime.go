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
	return renderFloorAndLive(w, r, p, resolve, nil)
}

// liveSetup carries the per-request live-session plumbing renderFloorAndLive
// needs to stream each live region's first render into the floor and mint a
// resumable session: the channel (for its session store), the token minted
// for this request, and the session those live regions mount into. The
// token/store mechanics live here so a live session started from the floor
// resumes over the same store the live routes read from.
type liveSetup struct {
	lc    LiveChannel
	token string
	sess  *LiveSession
}

// renderLiveFirst mounts and renders a live region's first state, recovering
// a panic exactly like renderRegion does for deferred regions so one
// misbehaving live region cannot take down the whole floor. A Mount error is
// treated the same way. ok reports whether tree and st are valid.
func renderLiveFirst(lr LiveRegion, ctx RenderContext) (tree Tree, st State, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	s, err := lr.Mount(ctx, nil)
	if err != nil {
		return Tree{}, nil, false
	}
	return lr.Render(s), s, true
}

// renderFloorAndLive is renderFloor's body, extended to also stream every
// live region's first render into the floor (tagged "live") and, when live
// is non-nil, register each live region's mounted state into live.sess and
// append the live manifest just before the shell tail so the client can
// resume the same session over the WebSocket. live is nil for the plain
// deferred/eager floor (renderFloor above), so that path is byte-for-byte
// unchanged.
func renderFloorAndLive(w http.ResponseWriter, r *http.Request, p *Page, resolve func(id string) fillTag, live *liveSetup) error {
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

	if live != nil {
		liveTag := fillTag{Strategy: "live", Trigger: ""}
		for _, id := range p.liveOrder { // page order: deterministic output
			lr := p.live[id]
			tree, st, ok := renderLiveFirst(lr, ctx)
			var html string
			if ok {
				html = tree.HTML()
				live.sess.set(id, &regionState{state: st, lastStatics: tree.Statics(), lastDynamics: tree.Dynamics()})
			} else {
				html = fmt.Sprintf(`<div data-q-error>region %q failed to render</div>`, id)
			}
			fill := fmt.Sprintf(
				`<div data-q-fill="%s" data-q-strategy="%s" data-q-trigger="%s">%s</div><script>window.__quicken&&window.__quicken.reveal(%s)</script>`,
				id, liveTag.Strategy, liveTag.Trigger, html, jsStringLiteral(id))
			if _, err := io.WriteString(w, fill); err != nil {
				return err
			}
			flush(w)
		}
		live.lc.store().Put(live.token, live.sess)
		if _, err := io.WriteString(w, liveManifestJSON(p, live.token)); err != nil {
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
// uses to label a region's fill. It is the sole resolver feeding the
// deferred/eager fills renderFloorAndLive streams from p.order (resolve is
// never called for a live region: those are tagged directly, since they are
// always Live and never go through Policy resolution in SP2).
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

// Serve mounts a page on mux at path. Each request resolves every
// deferred/eager region's cadence.Strategy from pol (nil pol = kind-inferred
// default) and streams the universal floor: shell + skeletons first, then
// every region's full content tagged with its strategy so the client shim
// reveals it accordingly. With scripting off the floor is the page.
//
// If the page has any live regions, their WebSocket/poll/event routes are
// mounted alongside path (once, at Serve time), and each request additionally
// mints a resume token and session, streams every live region's first render
// into the floor tagged "live", registers each into the session, and appends
// the live manifest, so a socket connecting with that token resumes the same
// state the floor just showed.
func Serve(mux *http.ServeMux, path string, p *Page, pol cadence.Policy) {
	if len(p.liveOrder) > 0 {
		var lc LiveChannel
		for route, h := range lc.liveRoutes(p) {
			mux.Handle(route, h)
		}
	}
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
		var live *liveSetup
		if len(p.liveOrder) > 0 {
			var lc LiveChannel
			token, err := newToken()
			if err != nil {
				http.Error(w, "session error", http.StatusInternalServerError)
				return
			}
			live = &liveSetup{
				lc:    lc,
				token: token,
				sess:  &LiveSession{regions: map[string]*regionState{}, outbox: make(chan serverMsg, 32)},
			}
		}
		_ = renderFloorAndLive(w, r, p, resolve, live)
	}))
}
