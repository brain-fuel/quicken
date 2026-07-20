package quicken

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"goforge.dev/cadence"
	"goforge.dev/goplus/std/result"
)

// renderFloor writes the universal fallback floor: the shell head, then every
// region's full content streamed into the document tail as a tagged fill with
// an inline reveal script, then the shell tail. With scripting off the fills
// stay visible after the body, so the page is fully readable; the shim
// relocates them into their slots per strategy.
func renderFloor(w http.ResponseWriter, r *http.Request, p *Page, resolve func(id string) cadence.Interpretation) error {
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

// renderFloorAndLive is renderFloor's body, extended to also stream every
// live region's first render into the floor (tagged "live") and, when live
// is non-nil, register each live region's mounted state into live.sess and
// append the live manifest just before the shell tail so the client can
// resume the same session over the WebSocket. live is nil for the plain
// deferred/eager floor (renderFloor above), so that path is byte-for-byte
// unchanged.
func renderFloorAndLive(w http.ResponseWriter, r *http.Request, p *Page, resolve func(id string) cadence.Interpretation, live *liveSetup) error {
	ctx := RenderContext{Ctx: r.Context(), R: r}
	frame := &Frame{page: p, ctx: ctx}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	doc := string(p.shell(frame))
	head, tail := splitBody(doc)

	if _, err := io.WriteString(w, head); err != nil {
		return err
	}
	flush(w)

	type floorResult struct {
		id   string
		html string
	}
	staticIDs := p.staticIDs()
	results := make(chan floorResult, len(staticIDs))
	for _, id := range staticIDs {
		id := id
		region, _ := p.staticRegion(id)
		go func() { results <- floorResult{id: id, html: renderRegion(region, ctx)} }()
	}

	var done <-chan struct{}
	if ctx.Ctx != nil {
		done = ctx.Ctx.Done()
	}
	got := make(map[string]string, len(staticIDs))
	for range staticIDs {
		select {
		case res := <-results:
			got[res.id] = res.html
		case <-done:
			return ctx.Ctx.Err()
		}
	}
	for _, id := range staticIDs { // page order: deterministic output
		strategy, trigger := wireTag(resolve(id))
		fill := fmt.Sprintf(
			`<div data-q-fill="%s" data-q-strategy="%s" data-q-trigger="%s">%s</div><script>window.__quicken&&window.__quicken.reveal(%s)</script>`,
			id, strategy, trigger, got[id], jsStringLiteral(id))
		if _, err := io.WriteString(w, fill); err != nil {
			return err
		}
		flush(w)
	}

	if live != nil {
		for _, id := range p.liveIDs() { // page order: deterministic output
			lr, _ := p.liveRegion(id)
			mounted := renderLiveFirst(lr, ctx)
			var html string
			result.Fold(mounted, result.ResultCases[mountedRegion, RenderFailure, bool]{
				Ok: func(m mountedRegion) bool {
					html = m.tree.HTML()
					live.sess.set(id, &regionState{state: m.state, lastStatics: m.tree.Statics(), lastDynamics: m.tree.Dynamics()})
					return true
				},
				Err: func(_ RenderFailure) bool {
					html = fmt.Sprintf(`<div data-q-error>region %q failed to render</div>`, id)
					return false
				},
			})
			fill := fmt.Sprintf(
				`<div data-q-fill="%s" data-q-strategy="%s" data-q-trigger="%s">%s</div><script>window.__quicken&&window.__quicken.reveal(%s)</script>`,
				id, "live", "", html, jsStringLiteral(id))
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
// interpreter, which SP2 does not yet provide. Live is only meaningful for a
// region registered with AddLive; a Policy that assigns Live to a plain
// region (registered with Add) is degraded to Eager, see below.
func strategyFor(p *Page, pol cadence.Policy, id string, ctx cadence.RequestContext) cadence.Interpretation {
	var s cadence.Strategy
	if pol == nil {
		match p.regionKind(id) {
		case cadence.Stateful():
			s = cadence.Live()
		case cadence.Plain():
			s = cadence.Deferred(cadence.Server(), cadence.OnLoad())
		}
	} else {
		s = pol.StrategyFor(id, ctx, nil)
	}
	return cadence.Interpret(p.regionKind(id), s, ctx)
}

// wireTag is the sole boundary that lowers semantic interpretations to the
// client shim's string protocol.
func wireTag(i cadence.Interpretation) (string, string) {
	match i {
	case cadence.Inline():
		return "eager", "onload"
	case cadence.AfterPaint(on):
		return "deferred", triggerName(on)
	case cadence.LiveTransport():
		return "live", ""
	case cadence.ClientCompute(_):
		return "eager", "onload"
	}
	return "eager", "onload"
}

func triggerName(on cadence.Trigger) string {
	match on {
	case cadence.OnLoad():
		return "onload"
	case cadence.OnVisible():
		return "onvisible"
	case cadence.OnHover():
		return "onhover"
	}
	return "onload"
}

// serveConfig is the resolved configuration Serve builds from its
// ServeOptions. Its zero value reproduces today's defaults exactly: a nil
// store resolves to the process-wide defaultStore (see LiveChannel.store),
// and a zero pollTimeout resolves to defaultPollTimeout (see
// LiveChannel.timeout).
type serveConfig struct {
	store       SessionStore
	pollTimeout time.Duration
}

// ServeOption configures Serve's live-session plumbing. Passing none
// reproduces today's defaults exactly.
type ServeOption func(*serveConfig)

// WithSessionStore supplies the SessionStore Serve mints live sessions into
// and resumes them from. Without this option Serve falls back to a
// process-wide in-memory store that never evicts a session; see the package
// doc's Production notes. Production deployments serving live regions should
// supply a bounded (TTL or LRU) store here.
func WithSessionStore(s SessionStore) ServeOption {
	return func(c *serveConfig) { c.store = s }
}

// WithPollTimeout sets how long the long-poll GET endpoint blocks waiting for
// the next queued message before responding 204. Without this option Serve
// uses defaultPollTimeout.
func WithPollTimeout(d time.Duration) ServeOption {
	return func(c *serveConfig) { c.pollTimeout = d }
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
//
// opts configures the live transport: WithSessionStore supplies the
// SessionStore (see its doc for why production deployments need this), and
// WithPollTimeout overrides the long-poll timeout. The single LiveChannel
// built from opts is used both to mount the live routes below and, per
// request, to mint each session, so a custom store is the one the mounted
// poll/WS/event handlers read AND the one the floor's first render writes
// into (LiveChannel.store resolves lc.Store, falling back to the process-wide
// defaultStore only when it is nil, so these two uses must share one lc value
// or a custom store would silently split from the routes reading it).
func Serve(mux *http.ServeMux, path string, p *Page, pol cadence.Policy, opts ...ServeOption) {
	var cfg serveConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	lc := LiveChannel{Store: cfg.store, pollTimeout: cfg.pollTimeout}

	if len(p.liveIDs()) > 0 {
		for route, h := range lc.liveRoutes(p) {
			mux.Handle(route, h)
		}
	}
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := cadence.RequestContext{Path: r.URL.Path}
		resolve := func(id string) cadence.Interpretation {
			return strategyFor(p, pol, id, ctx)
		}
		var live *liveSetup
		if len(p.liveIDs()) > 0 {
			token, err := newToken()
			if err != nil {
				http.Error(w, "session error", http.StatusInternalServerError)
				return
			}
			live = &liveSetup{
				lc:    lc,
				token: token,
				sess:  &LiveSession{regions: map[string]*regionState{}, outbox: make(chan ServerMessage, 32)},
			}
		}
		_ = renderFloorAndLive(w, r, p, resolve, live)
	}))
}
