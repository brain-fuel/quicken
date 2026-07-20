package quicken

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"goforge.dev/cadence"
	"goforge.dev/goplus/std/result"
)

// LiveChannel is the live transport: a WebSocket carries the first render and
// every subsequent fine-grained patch. A zero value uses a process-wide
// in-memory session store that never evicts a session; set Store to supply
// your own bounded (TTL or LRU) store for production. Serve builds and owns
// the LiveChannel for a page; callers configure it through Serve's
// WithSessionStore and WithPollTimeout options rather than constructing a
// LiveChannel directly.
//
// pollTimeout controls how long the long-poll GET endpoint blocks waiting for
// the next queued message before responding 204. Zero means a default (see
// defaultPollTimeout in longpoll.go).
type LiveChannel struct {
	Store       SessionStore
	pollTimeout time.Duration
}

var defaultStoreOnce sync.Once
var defaultStore SessionStore

func (lc LiveChannel) store() SessionStore {
	if lc.Store != nil {
		return lc.Store
	}
	defaultStoreOnce.Do(func() { defaultStore = NewMemoryStore() })
	return defaultStore
}

const liveBase = "/_live"

// liveBasePath is the base path for a page's live endpoints: /_live for an
// unnamed page, or /_live/<name> when the page is named. The WebSocket, poll,
// and event endpoints are siblings mounted at this base plus "/ws", "/poll",
// and "/event" respectively.
func liveBasePath(name string) string {
	if name == "" {
		return liveBase
	}
	return liveBase + "/" + name
}

// liveWSPath is the WebSocket endpoint for a page: /_live/ws for an unnamed
// page, or /_live/<name>/ws when the page is named.
func liveWSPath(name string) string {
	return liveBasePath(name) + "/ws"
}

// liveManifestJSON builds the live manifest the client shim reads to resume a
// session: the WebSocket path, the resume token, and every live region id, as
// a `<script type="application/json" data-q-live>` element. It is the single
// place this payload is produced, so the manifest a page load embeds always
// matches the routes Serve mounts.
func liveManifestJSON(p *Page, token string) string {
	ids := p.liveIDs()
	manifest, err := json.Marshal(map[string]any{
		"ws":    liveWSPath(p.name),
		"token": token,
		"ids":   ids,
	})
	if err != nil {
		// ids is a []string and token is a string: this cannot fail.
		manifest = []byte(`{}`)
	}
	return `<script type="application/json" data-q-live>` + string(manifest) + `</script>`
}

// liveRoutes builds the WebSocket endpoint and the long-poll fallback
// endpoints for this page. Serve mounts these alongside the page whenever the
// page has live regions.
func (lc LiveChannel) liveRoutes(p *Page) map[string]http.Handler {
	return map[string]http.Handler{
		liveWSPath(p.name):    lc.wsHandler(p),
		livePollPath(p.name):  lc.pollHandler(p),
		liveEventPath(p.name): lc.eventHandler(p),
	}
}

func (lc LiveChannel) wsHandler(p *Page) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrade(w, r)
		if err != nil {
			return
		}
		defer conn.Close()
		ctx := RenderContext{Ctx: r.Context(), R: r}
		lc.serve(conn, p, ctx)
	})
}

// serve runs one connection: resume, then an event loop. It returns when the
// connection errors or closes; the caller closes conn.
func (lc LiveChannel) serve(conn *wsConn, p *Page, ctx RenderContext) {
	_, first, err := conn.ReadMessage()
	if err != nil {
		return
	}
	msg, err := decodeClient(first)
	if err != nil {
		lc.send(conn, errorMsg("", "expected resume"))
		return
	}
	var token string
	match msg {
	case ResumeMessage(t):
		token = t
	case EventMessage(_, _, _, _):
		lc.send(conn, errorMsg("", "expected resume"))
		return
	}
	sess, ok := lc.store().Get(token)
	if !ok {
		lc.send(conn, errorMsg("", "unknown session"))
		return
	}
	for _, lr := range p.liveRegions() {
		var rendered result.Result[Tree, RenderFailure]
		found := sess.withRegion(lr.ID(), func(rs *regionState) {
			rendered = safeRender(lr, rs.state)
		})
		if !found {
			continue
		}
		msg := result.Fold(rendered, result.ResultCases[Tree, RenderFailure, ServerMessage]{
			Ok: func(tree Tree) ServerMessage {
				sess.withRegion(lr.ID(), func(rs *regionState) {
					rs.lastStatics = tree.Statics()
					rs.lastDynamics = tree.Dynamics()
				})
				return firstMsg(lr.ID(), tree)
			},
			Err: func(failure RenderFailure) ServerMessage { return errorMsg(lr.ID(), failure.Error()) },
		})
		if err := lc.send(conn, msg); err != nil {
			return
		}
	}
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		m, err := decodeClient(raw)
		if err != nil {
			continue
		}
		match m {
		case EventMessage(region, event, payload, _):
			if err := lc.handleEvent(conn, p, ctx, sess, region, event, payload); err != nil {
				return
			}
		case ResumeMessage(_):
		}
	}
}

func (lc LiveChannel) handleEvent(conn *wsConn, p *Page, ctx RenderContext, sess *LiveSession, region, event string, payload Payload) error {
	lr, ok := p.liveRegion(region)
	if !ok {
		return nil
	}
	var out ServerMessage
	found := sess.withRegion(region, func(rs *regionState) {
		out = lc.applyEvent(lr, ctx, rs, region, event, payload)
	})
	if !found {
		return nil
	}
	return lc.send(conn, out)
}

// applyEvent runs HandleEvent then Render and diffs, recovering panics into an
// error message. It updates rs in place on success.
func (lc LiveChannel) applyEvent(lr LiveRegion, ctx RenderContext, rs *regionState, region, event string, payload Payload) (out ServerMessage) {
	defer func() {
		if rec := recover(); rec != nil {
			out = errorMsg(region, "region panicked")
		}
	}()
	newState, err := lr.HandleEvent(ctx, event, payload, rs.state)
	if err != nil {
		return errorMsg(region, err.Error())
	}
	tree := lr.Render(newState)
	prev := Slots(rs.lastStatics, rs.lastDynamics)
	rs.state = newState
	rs.lastStatics = tree.Statics()
	rs.lastDynamics = tree.Dynamics()
	return cadence.TreeDiffFold(tree.Diff(prev), cadence.TreeDiffCases[ServerMessage]{
		Unchanged: func() ServerMessage { return patchMsg(region, map[int]string{}) },
		DynamicPatch: func(changed map[int]string) ServerMessage { return patchMsg(region, changed) },
		Replace: func(replacement Tree) ServerMessage { return fullMsg(region, replacement) },
	})
}

func (lc LiveChannel) send(conn *wsConn, m ServerMessage) error {
	b, err := encodeServer(m)
	if err != nil {
		return err
	}
	return conn.WriteText(b)
}
