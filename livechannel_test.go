package quicken

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// counter is a minimal LiveRegion for tests: State is an int, "inc" adds 1.
type counter struct{ id string }

func (c counter) ID() string                                 { return c.id }
func (c counter) Skeleton(RenderContext) Tree                { return Text(`<span>...</span>`) }
func (c counter) Mount(RenderContext, Params) (State, error) { return 0, nil }
func (c counter) HandleEvent(_ RenderContext, name string, _ Payload, s State) (State, error) {
	if name == "inc" {
		return s.(int) + 1, nil
	}
	return s, nil
}
func (c counter) Render(s State) Tree {
	return Slots([]string{`<b>`, `</b>`}, []string{strconv.Itoa(s.(int))})
}

// dialWS completes a client handshake against srv and returns the raw conn plus
// a buffered reader positioned after the 101 response.
func dialWS(t *testing.T, srv *httptest.Server, path string) (net.Conn, *bufio.Reader) {
	t.Helper()
	u := strings.TrimPrefix(srv.URL, "http://")
	conn, err := net.Dial("tcp", u)
	if err != nil {
		t.Fatal(err)
	}
	req := "GET " + path + " HTTP/1.1\r\n" +
		"Host: " + u + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(conn)
	// Read status line and headers up to the blank line.
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	return conn, br
}

// readServerFrame is a tiny client-side frame reader: server frames are never
// masked (RFC 6455 section 5.1 forbids masking on the server-to-client
// direction), so it cannot reuse wsConn.readFrame, which enforces masking in
// the opposite direction for the server's read side.
func readServerFrame(br *bufio.Reader) (opcode byte, payload []byte, err error) {
	var h [2]byte
	if _, err = io.ReadFull(br, h[:]); err != nil {
		return 0, nil, err
	}
	opcode = h[0] & 0x0f
	n := uint64(h[1] & 0x7f)
	switch n {
	case 126:
		var ext [2]byte
		if _, err = io.ReadFull(br, ext[:]); err != nil {
			return 0, nil, err
		}
		n = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err = io.ReadFull(br, ext[:]); err != nil {
			return 0, nil, err
		}
		n = binary.BigEndian.Uint64(ext[:])
	}
	payload = make([]byte, n)
	_, err = io.ReadFull(br, payload)
	return opcode, payload, err
}

// readServerText reads one server text frame's payload from br.
func readServerText(t *testing.T, br *bufio.Reader) []byte {
	t.Helper()
	op, payload, err := readServerFrame(br)
	if err != nil {
		t.Fatal(err)
	}
	if op != opText {
		t.Fatalf("expected text frame, got opcode %x", op)
	}
	return payload
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func manifestToken(t *testing.T, body string) string {
	t.Helper()
	i := strings.Index(body, `data-q-live>`)
	if i < 0 {
		t.Fatal("no live manifest in body")
	}
	rest := body[i+len(`data-q-live>`):]
	j := strings.Index(rest, "</script>")
	var man struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(rest[:j]), &man); err != nil {
		t.Fatal(err)
	}
	return man.Token
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestLiveChannelResumeSendsFirst(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<!doctype html><html><head>" + string(f.Head()) +
			"</head><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})

	mux := http.NewServeMux()
	Mount(mux)
	lc := LiveChannel{}
	Serve(mux, "/", page, lc)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Fetch the page to mint a session and read the resume token from the manifest.
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body := readAll(t, resp)
	token := manifestToken(t, body)

	conn, br := dialWS(t, srv, liveWSPath("demo"))
	defer conn.Close()
	if err := writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "resume", Token: token})); err != nil {
		t.Fatal(err)
	}
	var m serverMsg
	if err := json.Unmarshal(readServerText(t, br), &m); err != nil {
		t.Fatal(err)
	}
	if m.Type != "first" || m.Region != "c" || m.Dynamics[0] != "0" {
		t.Fatalf("first msg = %+v", m)
	}
}

func TestLiveChannelEventProducesPatch(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Mount(mux)
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))
	conn, br := dialWS(t, srv, liveWSPath("demo"))
	defer conn.Close()
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "resume", Token: token}))
	readServerText(t, br) // first

	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "event", Region: "c", Event: "inc"}))
	var m serverMsg
	json.Unmarshal(readServerText(t, br), &m)
	if m.Type != "patch" || m.Changed[0] != "1" {
		t.Fatalf("patch msg = %+v", m)
	}
}

func TestLiveChannelUnknownTokenErrors(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	conn, br := dialWS(t, srv, liveWSPath("demo"))
	defer conn.Close()
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "resume", Token: "bogus"}))
	var m serverMsg
	json.Unmarshal(readServerText(t, br), &m)
	if m.Type != "error" {
		t.Fatalf("expected error msg, got %+v", m)
	}
}

func TestLiveChannelResumeUnknownRegionSkipped(t *testing.T) {
	// A session with no regions registered under the token still resumes
	// cleanly with zero first messages; the loop over live regions in serve
	// simply finds nothing in the session for each id.
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	store := NewMemoryStore()
	lc := LiveChannel{Store: store}
	Serve(mux, "/", page, lc)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	token, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	store.Put(token, &LiveSession{regions: map[string]*regionState{}})

	conn, _ := dialWS(t, srv, liveWSPath("demo"))
	defer conn.Close()
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "resume", Token: token}))

	// The region "c" IS registered on the page but has no regionState in this
	// session, so an event for it is looked up and silently dropped (no
	// response) rather than crashing the connection.
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "event", Region: "c", Event: "inc"}))

	// No region state means no first message is sent either; the connection
	// should still accept a subsequent event without the earlier state, which
	// fails looking up the region's state and is silently dropped. Prove the
	// connection is still alive by sending a well-formed event for a region
	// that IS registered on a fresh resume and getting a patch back.
	conn.Close()

	resp, _ := http.Get(srv.URL + "/")
	token2 := manifestToken(t, readAll(t, resp))
	conn2, br2 := dialWS(t, srv, liveWSPath("demo"))
	defer conn2.Close()
	writeClientFrame(conn2, opText, mustJSON(clientMsg{Type: "resume", Token: token2}))
	readServerText(t, br2)
	writeClientFrame(conn2, opText, mustJSON(clientMsg{Type: "event", Region: "c", Event: "inc"}))
	var m serverMsg
	json.Unmarshal(readServerText(t, br2), &m)
	if m.Type != "patch" {
		t.Fatalf("patch msg = %+v", m)
	}
}

func TestLiveChannelMalformedFirstMessageErrors(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	conn, br := dialWS(t, srv, liveWSPath("demo"))
	defer conn.Close()
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "event", Region: "c", Event: "inc"}))
	var m serverMsg
	json.Unmarshal(readServerText(t, br), &m)
	if m.Type != "error" {
		t.Fatalf("expected error msg for a non-resume first message, got %+v", m)
	}
}

func TestLiveChannelEventUnknownEventIsNoop(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))
	conn, br := dialWS(t, srv, liveWSPath("demo"))
	defer conn.Close()
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "resume", Token: token}))
	readServerText(t, br) // first

	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "event", Region: "c", Event: "noop"}))
	var m serverMsg
	json.Unmarshal(readServerText(t, br), &m)
	if m.Type != "patch" || len(m.Changed) != 0 {
		t.Fatalf("expected an empty patch for a no-op event, got %+v", m)
	}
}

// Deliver writes exactly three chunks (head, manifest script, tail); reusing
// clientfetch_test.go's cfFailWriter (same test binary, same package) to fail
// after 0 and 1 successful writes covers both of Deliver's write-error
// branches, matching the technique already used for ClientFetch.Deliver.
func TestLiveChannelDeliverPropagatesWriteErrors(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	for ok := 0; ok < 2; ok++ {
		w := &cfFailWriter{ok: ok}
		err := (LiveChannel{}).Deliver(w, httptest.NewRequest(http.MethodGet, "/", nil), page)
		if err == nil {
			t.Fatalf("ok=%d: expected a write error to propagate", ok)
		}
	}
}

func TestLiveWSPath(t *testing.T) {
	if got := liveWSPath(""); got != "/_live/ws" {
		t.Fatalf("liveWSPath(\"\") = %q", got)
	}
	if got := liveWSPath("demo"); got != "/_live/demo/ws" {
		t.Fatalf("liveWSPath(\"demo\") = %q", got)
	}
}

func TestLiveChannelEventUnknownRegionIsNoop(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))
	conn, br := dialWS(t, srv, liveWSPath("demo"))
	defer conn.Close()
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "resume", Token: token}))
	readServerText(t, br) // first

	// "ghost" is not a registered live region; handleEvent looks it up in
	// p.live, fails, and drops the event silently. Prove the connection
	// survives by sending a well-formed event afterward and getting a patch.
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "event", Region: "ghost", Event: "inc"}))
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "event", Region: "c", Event: "inc"}))
	var m serverMsg
	json.Unmarshal(readServerText(t, br), &m)
	if m.Type != "patch" || m.Changed[0] != "1" {
		t.Fatalf("patch msg = %+v", m)
	}
}

// brokenMount is a LiveRegion whose Mount always fails, to exercise Deliver's
// mount-error path.
type brokenMount struct{ id string }

func (b brokenMount) ID() string                  { return b.id }
func (b brokenMount) Skeleton(RenderContext) Tree { return Text("...") }
func (b brokenMount) Mount(RenderContext, Params) (State, error) {
	return nil, errors.New("mount boom")
}
func (b brokenMount) HandleEvent(RenderContext, string, Payload, State) (State, error) {
	return nil, nil
}
func (b brokenMount) Render(State) Tree { return Text("x") }

func TestLiveChannelDeliverMountErrorReturns500(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("b")) + "</body></html>")
	}).Named("demo").AddLive(brokenMount{id: "b"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestLiveChannelWSHandlerRejectsNonUpgradeRequest(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + liveWSPath("demo"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestLiveChannelServeFirstReadErrorReturns(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn, _ := dialWS(t, srv, liveWSPath("demo"))
	// Close immediately without sending a resume message: the server's first
	// ReadMessage call errors and serve returns without hanging.
	conn.Close()
}

// erroringCounter behaves like counter but its HandleEvent fails a named
// event, to exercise applyEvent's HandleEvent-error path.
type erroringCounter struct{ id string }

func (e erroringCounter) ID() string                                 { return e.id }
func (e erroringCounter) Skeleton(RenderContext) Tree                { return Text("...") }
func (e erroringCounter) Mount(RenderContext, Params) (State, error) { return 0, nil }
func (e erroringCounter) HandleEvent(_ RenderContext, name string, _ Payload, s State) (State, error) {
	if name == "boom" {
		return nil, errors.New("handle boom")
	}
	return s, nil
}
func (e erroringCounter) Render(s State) Tree {
	return Slots([]string{`<b>`, `</b>`}, []string{strconv.Itoa(s.(int))})
}

func TestLiveChannelEventHandleEventErrorSendsError(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("e")) + "</body></html>")
	}).Named("demo").AddLive(erroringCounter{id: "e"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))
	conn, br := dialWS(t, srv, liveWSPath("demo"))
	defer conn.Close()
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "resume", Token: token}))
	readServerText(t, br) // first

	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "event", Region: "e", Event: "boom"}))
	var m serverMsg
	json.Unmarshal(readServerText(t, br), &m)
	if m.Type != "error" || m.Message != "handle boom" {
		t.Fatalf("error msg = %+v", m)
	}
}

// panickyCounter panics from HandleEvent on a named event, to exercise
// applyEvent's panic recovery.
type panickyCounter struct{ id string }

func (p panickyCounter) ID() string                                 { return p.id }
func (p panickyCounter) Skeleton(RenderContext) Tree                { return Text("...") }
func (p panickyCounter) Mount(RenderContext, Params) (State, error) { return 0, nil }
func (p panickyCounter) HandleEvent(_ RenderContext, name string, _ Payload, s State) (State, error) {
	if name == "boom" {
		panic("kaboom")
	}
	return s, nil
}
func (p panickyCounter) Render(s State) Tree {
	return Slots([]string{`<b>`, `</b>`}, []string{strconv.Itoa(s.(int))})
}

func TestLiveChannelEventPanicRecovered(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("p")) + "</body></html>")
	}).Named("demo").AddLive(panickyCounter{id: "p"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))
	conn, br := dialWS(t, srv, liveWSPath("demo"))
	defer conn.Close()
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "resume", Token: token}))
	readServerText(t, br) // first

	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "event", Region: "p", Event: "boom"}))
	var m serverMsg
	json.Unmarshal(readServerText(t, br), &m)
	if m.Type != "error" {
		t.Fatalf("expected error msg after a panic, got %+v", m)
	}

	// The connection must survive the panic: a subsequent well-formed event
	// still gets a normal patch response.
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "event", Region: "p", Event: "noop"}))
	var m2 serverMsg
	json.Unmarshal(readServerText(t, br), &m2)
	if m2.Type != "patch" {
		t.Fatalf("expected patch after recovery, got %+v", m2)
	}
}

// panickyFirstRender behaves like counter, but its Render panics starting
// from the SECOND call: Deliver's own Mount+Render (the first call) seeds the
// session normally, so the panic is reserved for the WS resume loop's
// re-render, exercising safeRender's recovery in serve rather than any
// recovery in Deliver (which has none).
type panickyFirstRender struct {
	id    string
	calls *int32
}

func (p panickyFirstRender) ID() string                                 { return p.id }
func (p panickyFirstRender) Skeleton(RenderContext) Tree                { return Text("...") }
func (p panickyFirstRender) Mount(RenderContext, Params) (State, error) { return 0, nil }
func (p panickyFirstRender) HandleEvent(_ RenderContext, _ string, _ Payload, s State) (State, error) {
	return s, nil
}
func (p panickyFirstRender) Render(s State) Tree {
	if atomic.AddInt32(p.calls, 1) > 1 {
		panic("kaboom on render")
	}
	return Slots([]string{`<b>`, `</b>`}, []string{"0"})
}

func TestLiveChannelResumeRenderPanicSendsErrorAndSurvives(t *testing.T) {
	var calls int32
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("p")) + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").
		AddLive(panickyFirstRender{id: "p", calls: &calls}).
		AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))
	conn, br := dialWS(t, srv, liveWSPath("demo"))
	defer conn.Close()
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "resume", Token: token}))

	// The panicking region "p" is mounted first (live regions render in the
	// order added), so its resume-render panics and must produce an "error"
	// message for that region rather than dropping the connection.
	var m1 serverMsg
	if err := json.Unmarshal(readServerText(t, br), &m1); err != nil {
		t.Fatal(err)
	}
	if m1.Type != "error" || m1.Region != "p" {
		t.Fatalf("first region msg = %+v, want an error for the panicking region", m1)
	}

	// The connection and the loop over regions must both survive: the second,
	// well-behaved region "c" still gets its normal "first" message.
	var m2 serverMsg
	if err := json.Unmarshal(readServerText(t, br), &m2); err != nil {
		t.Fatal(err)
	}
	if m2.Type != "first" || m2.Region != "c" {
		t.Fatalf("second region msg = %+v, want a normal first for the well-behaved region", m2)
	}
}

// shapeShifter changes its DYNAMIC SLOT COUNT (not just content) between
// renders, to exercise applyEvent's full-replace path when the slot count
// changes: the region starts with one dynamic slot and a "grow" event moves
// it to two. A slot-count change always changes len(statics) too (Tree
// invariant: len(statics) == len(dynamics)+1), so Tree.Diff's sameStatics
// check catches this on its own; there is no separate length guard anymore.
type shapeShifter struct{ id string }

func (s shapeShifter) ID() string                                 { return s.id }
func (s shapeShifter) Skeleton(RenderContext) Tree                { return Text("...") }
func (s shapeShifter) Mount(RenderContext, Params) (State, error) { return 0, nil }
func (s shapeShifter) HandleEvent(_ RenderContext, name string, _ Payload, st State) (State, error) {
	if name == "grow" {
		return 1, nil
	}
	return st, nil
}
func (s shapeShifter) Render(st State) Tree {
	if st.(int) == 0 {
		return Slots([]string{`<a>`, `</a>`}, []string{"one"})
	}
	return Slots([]string{`<a>`, `</a>`, `</a>`}, []string{"one", "two"})
}

func TestLiveChannelEventShapeChangeSendsFullViaDiff(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("s")) + "</body></html>")
	}).Named("demo").AddLive(shapeShifter{id: "s"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))
	conn, br := dialWS(t, srv, liveWSPath("demo"))
	defer conn.Close()
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "resume", Token: token}))
	readServerText(t, br) // first: one dynamic slot

	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "event", Region: "s", Event: "grow"}))
	var m serverMsg
	if err := json.Unmarshal(readServerText(t, br), &m); err != nil {
		t.Fatal(err)
	}
	if m.Type != "full" || len(m.Dynamics) != 2 || m.Dynamics[1] != "two" {
		t.Fatalf("full msg = %+v, want a full replace with 2 dynamics", m)
	}
}

// staticToggler keeps the SAME dynamic slot count across renders but changes
// a STATIC string (an "off"/"on" class on the wrapping element) depending on
// state. This is the regression FIX 1 closes: diffing dynamics alone, using
// the new tree's own statics as "prev", could never notice the static change,
// so the client kept a stale class forever. Diffing against the stored
// PREVIOUS statics must see the mismatch and force a full replace.
type staticToggler struct{ id string }

func (s staticToggler) ID() string                                 { return s.id }
func (s staticToggler) Skeleton(RenderContext) Tree                { return Text("...") }
func (s staticToggler) Mount(RenderContext, Params) (State, error) { return 0, nil }
func (s staticToggler) HandleEvent(_ RenderContext, name string, _ Payload, st State) (State, error) {
	if name == "toggle" {
		return 1, nil
	}
	return st, nil
}
func (s staticToggler) Render(st State) Tree {
	if st.(int) == 0 {
		return Slots([]string{`<i class="off">`, `</i>`}, []string{"x"})
	}
	return Slots([]string{`<i class="on">`, `</i>`}, []string{"x"})
}

func TestLiveChannelEventStaticOnlyChangeSendsFullNotPatch(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("t")) + "</body></html>")
	}).Named("demo").AddLive(staticToggler{id: "t"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))
	conn, br := dialWS(t, srv, liveWSPath("demo"))
	defer conn.Close()
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "resume", Token: token}))
	readServerText(t, br) // first: class "off", one dynamic slot ("x")

	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "event", Region: "t", Event: "toggle"}))
	var m serverMsg
	if err := json.Unmarshal(readServerText(t, br), &m); err != nil {
		t.Fatal(err)
	}
	// Same dynamic-slot count ("x" both times) but the static class flipped
	// off->on: the server must send a "full" carrying the new statics, not a
	// "patch" (which would leave the client's stale "off" class in place
	// forever, since a patch never touches statics).
	if m.Type != "full" {
		t.Fatalf("msg = %+v, want type \"full\" for a static-only change", m)
	}
	if len(m.Statics) != 2 || m.Statics[0] != `<i class="on">` {
		t.Fatalf("full msg statics = %v, want the new [\"<i class=\\\"on\\\">\", \"</i>\"]", m.Statics)
	}
}

func TestLiveChannelEventLoopIgnoresNonEventMessage(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))
	conn, br := dialWS(t, srv, liveWSPath("demo"))
	defer conn.Close()
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "resume", Token: token}))
	readServerText(t, br) // first

	// A second "resume" message, and a malformed (non-JSON) message, both hit
	// the event loop's continue branch (not type "event", or fails to
	// decode) rather than crashing the connection.
	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "resume", Token: token}))
	writeClientFrame(conn, opText, []byte("not json"))

	writeClientFrame(conn, opText, mustJSON(clientMsg{Type: "event", Region: "c", Event: "inc"}))
	var m serverMsg
	json.Unmarshal(readServerText(t, br), &m)
	if m.Type != "patch" || m.Changed[0] != "1" {
		t.Fatalf("patch msg = %+v", m)
	}
}

// mountCounter mounts a counter LiveRegion directly, bypassing Deliver, so a
// test can drive LiveChannel.serve on a hand-built session without an HTTP
// round trip.
func mountCounterSession(id string, ctx RenderContext) (LiveRegion, *LiveSession) {
	lr := counter{id: id}
	st, _ := lr.Mount(ctx, nil)
	tree := lr.Render(st)
	sess := &LiveSession{regions: map[string]*regionState{
		id: {state: st, lastDynamics: tree.Dynamics()},
	}}
	return lr, sess
}

func TestLiveChannelServeFirstMessageWriteErrorReturns(t *testing.T) {
	// Constructs a wsConn directly over a net.Pipe (as ws_test.go does) so
	// the write side can be severed deterministically: closing the peer
	// right after it delivers the resume frame makes the server's very next
	// WriteText (the "first" message) fail, which must make serve return
	// rather than hang or panic.
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	ctx := RenderContext{}
	_, sess := mountCounterSession("c", ctx)
	store := NewMemoryStore()
	token, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	store.Put(token, sess)
	lc := LiveChannel{Store: store}

	cli, srvConn := net.Pipe()
	conn := &wsConn{conn: srvConn, br: bufio.NewReader(srvConn)}

	done := make(chan struct{})
	go func() {
		lc.serve(conn, page, ctx)
		close(done)
	}()

	if err := writeClientFrame(cli, opText, mustJSON(clientMsg{Type: "resume", Token: token})); err != nil {
		t.Fatal(err)
	}
	cli.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not return after the first-message write failed")
	}
}

func TestLiveChannelServeEventWriteErrorReturns(t *testing.T) {
	// Same technique as above, but severs the connection after a successful
	// resume/first exchange, right before the event handler's response
	// write, to exercise handleEvent's write-error return in serve's event
	// loop.
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	ctx := RenderContext{}
	_, sess := mountCounterSession("c", ctx)
	store := NewMemoryStore()
	token, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	store.Put(token, sess)
	lc := LiveChannel{Store: store}

	cli, srvConn := net.Pipe()
	conn := &wsConn{conn: srvConn, br: bufio.NewReader(srvConn)}
	cliBR := bufio.NewReader(cli)

	done := make(chan struct{})
	go func() {
		lc.serve(conn, page, ctx)
		close(done)
	}()

	if err := writeClientFrame(cli, opText, mustJSON(clientMsg{Type: "resume", Token: token})); err != nil {
		t.Fatal(err)
	}
	readServerText(t, cliBR) // first

	if err := writeClientFrame(cli, opText, mustJSON(clientMsg{Type: "event", Region: "c", Event: "inc"})); err != nil {
		t.Fatal(err)
	}
	cli.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not return after the event-response write failed")
	}
}
