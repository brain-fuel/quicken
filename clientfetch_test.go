package quicken

import (
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func cfPage(name string) *Page {
	shell := func(f *Frame) template.HTML {
		return template.HTML("<!doctype html><html><head>" + string(f.Head()) +
			"</head><body>" + string(f.Slot("alpha")) + string(f.Slot("beta")) +
			"</body></html>")
	}
	p := NewPage(shell).
		Add(RegionFunc("alpha",
			func(RenderContext) Tree { return Text("<i>loading alpha</i>") },
			func(RenderContext) Tree { return Text("<p>ALPHA CONTENT</p>") })).
		Add(RegionFunc("beta",
			func(RenderContext) Tree { return Text("<i>loading beta</i>") },
			func(RenderContext) Tree { return Text("<p>BETA CONTENT</p>") }))
	return p.Named(name)
}

func TestClientFetchDeliverHasSkeletonsAndManifestButNoContent(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := (ClientFetch{}).Deliver(rec, httptest.NewRequest(http.MethodGet, "/", nil), cfPage("demo")); err != nil {
		t.Fatalf("Deliver error: %v", err)
	}
	body := rec.Body.String()
	for _, want := range []string{`id="q-slot-alpha"`, "data-q-pending", "loading alpha", "data-q-manifest", `"page":"demo"`, `"alpha"`, `"beta"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q\n%s", want, body)
		}
	}
	// The initial response must NOT contain any region's real content.
	for _, forbidden := range []string{"ALPHA CONTENT", "BETA CONTENT"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("initial ClientFetch response leaked region content %q", forbidden)
		}
	}
}

func TestClientFetchRoutesRenderIndividualRegions(t *testing.T) {
	routes := (ClientFetch{}).Routes(cfPage("demo"))
	if len(routes) != 2 {
		t.Fatalf("routes = %d, want 2", len(routes))
	}
	h, ok := routes["/_regions/demo/alpha"]
	if !ok {
		t.Fatalf("missing route /_regions/demo/alpha; got %v", keysOf(routes))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_regions/demo/alpha", nil))
	if !strings.Contains(rec.Body.String(), "ALPHA CONTENT") {
		t.Fatalf("route body = %q", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("route content-type = %q", ct)
	}
}

func TestClientFetchRoutePanicIsErrorCard(t *testing.T) {
	shell := func(f *Frame) template.HTML { return template.HTML("<html><body>" + string(f.Slot("boom")) + "</body></html>") }
	p := NewPage(shell).Add(RegionFunc("boom",
		func(RenderContext) Tree { return Text("sk") },
		func(RenderContext) Tree { panic("boom") }))
	h := (ClientFetch{}).Routes(p)["/_regions/boom"]
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_regions/boom", nil))
	if !strings.Contains(rec.Body.String(), "data-q-error") {
		t.Fatalf("panic route body = %q", rec.Body.String())
	}
}

func TestClientFetchUnnamedManifestAndRoutes(t *testing.T) {
	p := cfPage("")
	rec := httptest.NewRecorder()
	if err := (ClientFetch{}).Deliver(rec, httptest.NewRequest(http.MethodGet, "/", nil), p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rec.Body.String(), `"page":""`) {
		t.Fatalf("unnamed manifest missing empty page: %s", rec.Body.String())
	}
	if _, ok := (ClientFetch{}).Routes(p)["/_regions/alpha"]; !ok {
		t.Fatal("unnamed route /_regions/alpha missing")
	}
}

func TestClientFetchEndToEndViaServe(t *testing.T) {
	mux := http.NewServeMux()
	Mount(mux)
	Serve(mux, "/", cfPage("demo"), ClientFetch{})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(rec.Body.String(), "data-q-manifest") {
		t.Fatal("served page missing manifest")
	}
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/_regions/demo/beta", nil))
	if !strings.Contains(rec2.Body.String(), "BETA CONTENT") {
		t.Fatalf("served region body = %q", rec2.Body.String())
	}
}

func keysOf(m map[string]http.Handler) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// cfFailWriter is an http.ResponseWriter whose Write fails after `ok`
// successful writes, to exercise Deliver's write-error branches. It does not
// implement io.StringWriter, so each io.WriteString is exactly one Write.
type cfFailWriter struct {
	hdr http.Header
	ok  int
	n   int
}

func (w *cfFailWriter) Header() http.Header {
	if w.hdr == nil {
		w.hdr = http.Header{}
	}
	return w.hdr
}
func (w *cfFailWriter) WriteHeader(int) {}
func (w *cfFailWriter) Write(b []byte) (int, error) {
	if w.n >= w.ok {
		return 0, errors.New("write failed")
	}
	w.n++
	return len(b), nil
}

// Deliver writes exactly three chunks (head, manifest, tail); failing after 0,
// 1, and 2 successful writes covers each of its write-error branches.
func TestClientFetchDeliverPropagatesWriteErrors(t *testing.T) {
	for ok := 0; ok < 3; ok++ {
		w := &cfFailWriter{ok: ok}
		err := (ClientFetch{}).Deliver(w, httptest.NewRequest(http.MethodGet, "/", nil), cfPage("demo"))
		if err == nil {
			t.Fatalf("ok=%d: expected a write error to propagate", ok)
		}
	}
}
