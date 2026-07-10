package quicken

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const shortTimeout = 50 * time.Millisecond

func TestLongPollFirstThenPatch(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))

	// First poll yields the initial render.
	pr, err := http.Get(srv.URL + livePollPath("demo") + "?token=" + token)
	if err != nil {
		t.Fatal(err)
	}
	var first serverMsg
	json.Unmarshal([]byte(readAll(t, pr)), &first)
	if first.Type != "first" || first.Dynamics[0] != "0" {
		t.Fatalf("first poll = %+v", first)
	}

	// Post an event.
	ev := mustJSON(clientMsg{Type: "event", Region: "c", Event: "inc", Token: token})
	er, err := http.Post(srv.URL+liveEventPath("demo"), "application/json", bytes.NewReader(ev))
	if err != nil {
		t.Fatal(err)
	}
	if er.StatusCode != http.StatusNoContent {
		t.Fatalf("event status = %d", er.StatusCode)
	}

	// Next poll yields the patch.
	pr2, _ := http.Get(srv.URL + livePollPath("demo") + "?token=" + token)
	var patch serverMsg
	json.Unmarshal([]byte(readAll(t, pr2)), &patch)
	if patch.Type != "patch" || patch.Changed[0] != "1" {
		t.Fatalf("patch poll = %+v", patch)
	}
}

func TestLongPollTimeoutIsNoContent(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{pollTimeout: shortTimeout})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))
	// Drain the first message.
	http.Get(srv.URL + livePollPath("demo") + "?token=" + token)
	// Second poll has nothing queued and must time out to 204.
	pr, _ := http.Get(srv.URL + livePollPath("demo") + "?token=" + token)
	if pr.StatusCode != http.StatusNoContent {
		t.Fatalf("timeout poll status = %d", pr.StatusCode)
	}
}

func TestLongPollUnknownTokenIs404(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	pr, _ := http.Get(srv.URL + livePollPath("demo") + "?token=nope")
	if pr.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown token poll status = %d", pr.StatusCode)
	}
}

// TestLongPollEventUnknownRegionIs404 exercises the eventHandler's region
// lookup miss (distinct from the token miss above).
func TestLongPollEventUnknownRegionIs404(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))

	ev := mustJSON(clientMsg{Type: "event", Region: "ghost", Event: "inc", Token: token})
	er, err := http.Post(srv.URL+liveEventPath("demo"), "application/json", bytes.NewReader(ev))
	if err != nil {
		t.Fatal(err)
	}
	if er.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown region event status = %d", er.StatusCode)
	}
}

// TestLongPollEventBadBodyIs400 exercises the malformed-body branch of
// eventHandler.
func TestLongPollEventBadBodyIs400(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	er, err := http.Post(srv.URL+liveEventPath("demo"), "application/json", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatal(err)
	}
	if er.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad body event status = %d", er.StatusCode)
	}
}

// TestLongPollUnnamedPagePath exercises livePollPath's unnamed-page branch
// (the Named test pages exercise the named branch elsewhere in this file).
func TestLongPollUnnamedPagePath(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))

	if got, want := livePollPath(""), "/_live/poll"; got != want {
		t.Fatalf("livePollPath(\"\") = %q, want %q", got, want)
	}
	pr, err := http.Get(srv.URL + livePollPath("") + "?token=" + token)
	if err != nil {
		t.Fatal(err)
	}
	var first serverMsg
	json.Unmarshal([]byte(readAll(t, pr)), &first)
	if first.Type != "first" {
		t.Fatalf("first poll on unnamed page = %+v", first)
	}
}

// TestLongPollEventUnknownTokenIs404 exercises the eventHandler's token
// lookup miss, distinct from the region-miss and poll-token-miss cases
// covered elsewhere.
func TestLongPollEventUnknownTokenIs404(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ev := mustJSON(clientMsg{Type: "event", Region: "c", Event: "inc", Token: "nope"})
	er, err := http.Post(srv.URL+liveEventPath("demo"), "application/json", bytes.NewReader(ev))
	if err != nil {
		t.Fatal(err)
	}
	if er.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown token event status = %d", er.StatusCode)
	}
}

// TestLongPollContextCancelUnblocks confirms a poll blocked on the outbox is
// released promptly by r.Context().Done() when the client cancels, rather
// than sitting until pollTimeout. pollTimeout is set well above the client's
// cancellation deadline, so a fast return here can only be explained by the
// context branch, not the timeout branch. The deferred srv.Close() also
// blocks until the handler goroutine actually returns, so a real leak (the
// handler never noticing cancellation) would hang this test instead of
// passing silently.
func TestLongPollContextCancelUnblocks(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{pollTimeout: 2 * time.Second})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))
	// Drain the first message so this poll blocks on an empty outbox.
	http.Get(srv.URL + livePollPath("demo") + "?token=" + token)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+livePollPath("demo")+"?token="+token, nil)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = http.DefaultClient.Do(req)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected client-side context cancellation error")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("cancellation took %v to unblock, want well under the 2s pollTimeout", elapsed)
	}
}

// TestLongPollOutboxDropsOldest forces the outbox past its buffered capacity
// (32) with nobody polling, then confirms the drop-oldest path in
// eventHandler actually dropped the oldest message rather than the newest:
// after posting 37 events with none consumed, exactly 5 are dropped, so the
// oldest surviving patch carries changed[0] == "6" (values 1..5 were
// evicted).
func TestLongPollOutboxDropsOldest(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))

	// Drain the "first" message so the outbox starts empty; subsequent
	// posts go straight through eventHandler's enqueue path.
	http.Get(srv.URL + livePollPath("demo") + "?token=" + token)

	const capacity = 32
	const overflow = 5
	for i := 0; i < capacity+overflow; i++ {
		ev := mustJSON(clientMsg{Type: "event", Region: "c", Event: "inc", Token: token})
		er, err := http.Post(srv.URL+liveEventPath("demo"), "application/json", bytes.NewReader(ev))
		if err != nil {
			t.Fatal(err)
		}
		if er.StatusCode != http.StatusNoContent {
			t.Fatalf("event %d status = %d", i, er.StatusCode)
		}
	}

	pr, _ := http.Get(srv.URL + livePollPath("demo") + "?token=" + token)
	var oldest serverMsg
	json.Unmarshal([]byte(readAll(t, pr)), &oldest)
	want := fmt.Sprintf("%d", overflow+1)
	if oldest.Type != "patch" || oldest.Changed[0] != want {
		t.Fatalf("oldest surviving patch = %+v, want changed[0] = %q", oldest, want)
	}
}

// TestLongPollFirstSendDrainsStaleOutboxMessages covers a client that posts
// enough events to fill the 32-slot outbox before ever polling: a stale
// patch already queued from an event POST used to be exactly what the first
// poll's per-region enqueue then had to push past, since nothing had
// consumed those patches yet. The fix drains any already-queued messages
// before building the fresh "first" batch, so the first poll still returns
// promptly with a "first" message (not a stale patch), and the outbox holds
// only what the first-send loop enqueued afterward.
func TestLongPollFirstSendDrainsStaleOutboxMessages(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	}).Named("demo").AddLive(counter{id: "c"})
	mux := http.NewServeMux()
	lc := LiveChannel{pollTimeout: 2 * time.Second}
	Serve(mux, "/", page, lc)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))

	sess, ok := lc.store().Get(token)
	if !ok {
		t.Fatal("session not found")
	}
	// Fill the outbox to capacity with stale patches, as an event POST would,
	// before the client ever polls (so markFirstSent has not fired yet).
	const capacity = 32
	for i := 0; i < capacity; i++ {
		sess.outbox <- serverMsg{Type: "patch", Region: "c", Changed: map[int]string{0: "stale"}}
	}

	start := time.Now()
	pr, err := http.Get(srv.URL + livePollPath("demo") + "?token=" + token)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Fatalf("first poll took %v with a full outbox, want well under the 2s pollTimeout", elapsed)
	}
	var m serverMsg
	json.Unmarshal([]byte(readAll(t, pr)), &m)
	if m.Type != "first" {
		t.Fatalf("first poll after a full outbox = %+v, want type \"first\" (stale patches must be superseded, not returned)", m)
	}
	if n := len(sess.outbox); n != 0 {
		t.Fatalf("outbox length after first poll = %d, want 0 (single region, first message already consumed)", n)
	}
}

// TestLongPollFirstSendRenderPanicSendsErrorAndSurvives is the long-poll
// counterpart of the WS resume panic-recovery test: the first poll's
// per-region enqueue loop must convert a panicking Render into an "error"
// message for that region instead of letting the panic escape the handler,
// and the well-behaved region alongside it must still get its "first".
func TestLongPollFirstSendRenderPanicSendsErrorAndSurvives(t *testing.T) {
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

	pr, err := http.Get(srv.URL + livePollPath("demo") + "?token=" + token)
	if err != nil {
		t.Fatal(err)
	}
	var m serverMsg
	json.Unmarshal([]byte(readAll(t, pr)), &m)
	if m.Type != "error" || m.Region != "p" {
		t.Fatalf("first poll = %+v, want an error for the panicking region", m)
	}

	pr2, err := http.Get(srv.URL + livePollPath("demo") + "?token=" + token)
	if err != nil {
		t.Fatal(err)
	}
	var m2 serverMsg
	json.Unmarshal([]byte(readAll(t, pr2)), &m2)
	if m2.Type != "first" || m2.Region != "c" {
		t.Fatalf("second poll = %+v, want a normal first for the well-behaved region", m2)
	}
}

// manyCounterIDs returns n distinct live-region ids, used to build a page
// with more live regions than the outbox's fixed 32-slot capacity.
func manyCounterIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("c%d", i)
	}
	return ids
}

// TestLongPollFirstSendManyRegionsUnblocksOnCancel reproduces the hang FIX A
// closes directly: with more live regions on a page than the outbox's fixed
// 32-slot capacity, the first poll's per-region enqueue loop fills the
// outbox and then has nowhere to put the remaining "first" messages, since
// nobody is reading. Before the fix, `sess.outbox <- fm` was a plain
// blocking send with no escape: a client disconnect does not release a
// blocked channel send, so the handler goroutine leaked forever. The fixed
// enqueue selects on r.Context().Done() too, so a canceled poll returns
// promptly instead. pollTimeout is set well above the client's cancellation
// deadline, so a fast return can only be explained by the context branch,
// not the timeout branch. The deferred srv.Close() blocks until the handler
// goroutine actually returns, so a real leak would hang this test instead of
// passing silently.
func TestLongPollFirstSendManyRegionsUnblocksOnCancel(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body></body></html>")
	}).Named("demo")
	for _, id := range manyCounterIDs(40) {
		page.AddLive(counter{id: id})
	}
	mux := http.NewServeMux()
	Serve(mux, "/", page, LiveChannel{pollTimeout: 2 * time.Second})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/")
	token := manifestToken(t, readAll(t, resp))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+livePollPath("demo")+"?token="+token, nil)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = http.DefaultClient.Do(req)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected client-side context cancellation error")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("cancellation took %v to unblock with 40 live regions over a 32-slot outbox, want well under the 2s pollTimeout", elapsed)
	}
}
