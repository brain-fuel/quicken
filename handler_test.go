package quicken

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestServeNilPolicyServesRegionContent asserts that Serve with a nil policy
// (the kind-inferred default) streams a region's real content into the floor.
func TestServeNilPolicyServesRegionContent(t *testing.T) {
	mux := http.NewServeMux()
	Serve(mux, "/", demoPage(), nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "ALPHA CONTENT") {
		t.Fatalf("Serve(nil) body missing region content: %q", body)
	}
}

// nonFlushResponseWriter implements http.ResponseWriter but deliberately does
// not implement http.Flusher, to cover flush's non-flusher branch.
type nonFlushResponseWriter struct {
	header http.Header
	body   strings.Builder
	status int
}

func newNonFlushResponseWriter() *nonFlushResponseWriter {
	return &nonFlushResponseWriter{header: http.Header{}}
}

func (w *nonFlushResponseWriter) Header() http.Header { return w.header }

func (w *nonFlushResponseWriter) Write(b []byte) (int, error) { return w.body.Write(b) }

func (w *nonFlushResponseWriter) WriteHeader(statusCode int) { w.status = statusCode }

// Compile-time guard: nonFlushResponseWriter must NOT implement http.Flusher.
var _ http.ResponseWriter = (*nonFlushResponseWriter)(nil)

func TestFlushNonFlusherWriterStillDeliversFullDocument(t *testing.T) {
	w := newNonFlushResponseWriter()
	// Guard against http.Flusher being satisfied accidentally.
	if _, ok := any(w).(http.Flusher); ok {
		t.Fatal("test double unexpectedly implements http.Flusher")
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := renderFloor(w, req, demoPage(), defaultResolve); err != nil {
		t.Fatalf("renderFloor error: %v", err)
	}
	body := w.body.String()
	for _, want := range []string{"ALPHA CONTENT", "BETA CONTENT", "</body></html>"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q with non-flusher writer: %q", want, body)
		}
	}
}

// failingAfterWriter fails the Nth call to Write (1-indexed) and succeeds on
// every call before it, so a test can target a specific io.WriteString call
// inside Deliver (the head write, or a later fill write) and exercise its
// error-return branch.
type failingAfterWriter struct {
	header  http.Header
	failAt  int
	writeNo int
}

func newFailingAfterWriter(failAt int) *failingAfterWriter {
	return &failingAfterWriter{header: http.Header{}, failAt: failAt}
}

func (w *failingAfterWriter) Header() http.Header { return w.header }

func (w *failingAfterWriter) Write(b []byte) (int, error) {
	w.writeNo++
	if w.writeNo == w.failAt {
		return 0, errors.New("simulated write failure")
	}
	return len(b), nil
}

func (w *failingAfterWriter) WriteHeader(int) {}

func TestRenderFloorReturnsErrorWhenHeadWriteFails(t *testing.T) {
	w := newFailingAfterWriter(1)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := renderFloor(w, req, demoPage(), defaultResolve); err == nil {
		t.Fatal("expected error from failed head write, got nil")
	}
}

func TestRenderFloorReturnsErrorWhenFillWriteFails(t *testing.T) {
	// The 1st Write is the shell head; the 2nd is the first region fill.
	w := newFailingAfterWriter(2)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := renderFloor(w, req, demoPage(), defaultResolve); err == nil {
		t.Fatal("expected error from failed fill write, got nil")
	}
}
