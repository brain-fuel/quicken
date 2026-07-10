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

// Deliver renders the shell with skeletons and a live manifest, mints a
// session, and mounts every live region so its state is ready when the socket
// connects. It does not stream any region's live HTML: the client renders
// "first" messages received over the socket, and the skeleton already in the
// document is the JS-off floor.
func (lc LiveChannel) Deliver(w http.ResponseWriter, r *http.Request, p *Page) error {
	ctx := RenderContext{Ctx: r.Context(), R: r}
	token, err := newToken()
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return err
	}
	sess := &LiveSession{regions: map[string]*regionState{}, outbox: make(chan serverMsg, 32)}
	ids := make([]string, 0, len(p.liveRegions()))
	for _, lr := range p.liveRegions() {
		st, err := lr.Mount(ctx, nil)
		if err != nil {
			http.Error(w, "mount error", http.StatusInternalServerError)
			return err
		}
		tree := lr.Render(st)
		sess.set(lr.ID(), &regionState{state: st, lastDynamics: tree.dynamics})
		ids = append(ids, lr.ID())
	}
	lc.store().Put(token, sess)

	doc := string(p.shell(&Frame{page: p, ctx: ctx}))
	head, tail := splitBody(doc)
	manifest, err := json.Marshal(map[string]any{
		"ws":    liveWSPath(p.name),
		"token": token,
		"ids":   ids,
	})
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(head)); err != nil {
		return err
	}
	if _, err := w.Write([]byte(`<script type="application/json" data-q-live>` + string(manifest) + `</script>`)); err != nil {
		return err
	}
	_, err = w.Write([]byte(tail))
	return err
}

// Routes mounts the WebSocket endpoint and the long-poll fallback endpoints
// for this page.
func (lc LiveChannel) Routes(p *Page) map[string]http.Handler {
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
		found := sess.withRegion(lr.ID(), func(rs *regionState) {
			tree = lr.Render(rs.state)
			rs.lastDynamics = tree.dynamics
		})
		if !found {
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

	if len(rs.lastDynamics) != len(tree.dynamics) {
		rs.state = newState
		rs.lastDynamics = tree.dynamics
		return fullMsg(m.Region, tree)
	}

	prev := Tree{statics: tree.statics, dynamics: rs.lastDynamics}
	changed, _ := tree.Diff(prev)
	rs.state = newState
	rs.lastDynamics = tree.dynamics
	return patchMsg(m.Region, changed)
}

func (lc LiveChannel) send(conn *wsConn, m serverMsg) error {
	b, err := encodeServer(m)
	if err != nil {
		return err
	}
	return conn.WriteText(b)
}
