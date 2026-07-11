package quicken

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// LiveChannel is the live transport: a WebSocket carries the first render and
// every subsequent fine-grained patch. A zero value uses a process-wide
// in-memory session store; set Store to supply your own.
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
	ids := make([]string, 0, len(p.liveOrder))
	ids = append(ids, p.liveOrder...)
	manifest, err := json.Marshal(map[string]any{
		"ws":    liveWSPath(p.name),
		"token": token,
		"ids":   ids,
	})
	if err != nil {
		// p.liveOrder is a []string and token is a string: this cannot fail.
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
	if err != nil || msg.Type != "resume" {
		lc.send(conn, errorMsg("", "expected resume"))
		return
	}
	sess, ok := lc.store().Get(msg.Token)
	if !ok {
		lc.send(conn, errorMsg("", "unknown session"))
		return
	}
	for _, lr := range p.liveRegions() {
		var tree Tree
		var renderOK bool
		found := sess.withRegion(lr.ID(), func(rs *regionState) {
			tree, renderOK = safeRender(lr, rs.state)
			if renderOK {
				rs.lastStatics = tree.Statics()
				rs.lastDynamics = tree.Dynamics()
			}
		})
		if !found {
			continue
		}
		if !renderOK {
			if err := lc.send(conn, errorMsg(lr.ID(), "region panicked")); err != nil {
				return
			}
			continue
		}
		if err := lc.send(conn, firstMsg(lr.ID(), tree)); err != nil {
			return
		}
	}
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		m, err := decodeClient(raw)
		if err != nil || m.Type != "event" {
			continue
		}
		if err := lc.handleEvent(conn, p, ctx, sess, m); err != nil {
			return
		}
	}
}

func (lc LiveChannel) handleEvent(conn *wsConn, p *Page, ctx RenderContext, sess *LiveSession, m clientMsg) error {
	lr, ok := p.live[m.Region]
	if !ok {
		return nil
	}
	var out serverMsg
	found := sess.withRegion(m.Region, func(rs *regionState) {
		out = lc.applyEvent(lr, ctx, rs, m)
	})
	if !found {
		return nil
	}
	return lc.send(conn, out)
}

// applyEvent runs HandleEvent then Render and diffs, recovering panics into an
// error message. It updates rs in place on success.
func (lc LiveChannel) applyEvent(lr LiveRegion, ctx RenderContext, rs *regionState, m clientMsg) (out serverMsg) {
	defer func() {
		if rec := recover(); rec != nil {
			out = errorMsg(m.Region, "region panicked")
		}
	}()
	newState, err := lr.HandleEvent(ctx, m.Event, m.Payload, rs.state)
	if err != nil {
		return errorMsg(m.Region, err.Error())
	}
	tree := lr.Render(newState)
	prev := Slots(rs.lastStatics, rs.lastDynamics)
	changed, full := tree.Diff(prev)
	rs.state = newState
	rs.lastStatics = tree.Statics()
	rs.lastDynamics = tree.Dynamics()
	if full {
		return fullMsg(m.Region, tree)
	}
	return patchMsg(m.Region, changed)
}

// safeRender renders a region, converting a panic into ok=false so the caller
// can emit an error message instead of dropping the connection. It exists
// because the WS resume loop and the long-poll first-send loop both call
// Render before any event has occurred, outside applyEvent's own panic
// recovery, and a panicking first render should not tear down the whole
// connection or poll.
func safeRender(lr LiveRegion, s State) (t Tree, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return lr.Render(s), true
}

func (lc LiveChannel) send(conn *wsConn, m serverMsg) error {
	b, err := encodeServer(m)
	if err != nil {
		return err
	}
	return conn.WriteText(b)
}
