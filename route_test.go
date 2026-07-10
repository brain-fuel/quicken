package quicken

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
)

func routeShell(*Frame) template.HTML { return "<html><body></body></html>" }

func TestNamedRejectsInvalid(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on invalid page name")
		}
	}()
	NewPage(routeShell).Named("bad name!")
}

func TestNamedAcceptsEmptyAndValid(t *testing.T) {
	p := NewPage(routeShell).Named("")
	if p.name != "" {
		t.Fatalf("empty name = %q", p.name)
	}
	p2 := NewPage(routeShell).Named("demo")
	if p2.name != "demo" {
		t.Fatalf("name = %q", p2.name)
	}
}

func TestRegionPath(t *testing.T) {
	if got := regionPath("", "alpha"); got != "/_regions/alpha" {
		t.Fatalf("unnamed regionPath = %q", got)
	}
	if got := regionPath("demo", "alpha"); got != "/_regions/demo/alpha" {
		t.Fatalf("named regionPath = %q", got)
	}
}

// stubRouteTransport is a minimal transport that also provides a route, so
// Serve's RouteProvider branch can be exercised without ClientFetch.
type stubRouteTransport struct{ hit *bool }

func (stubRouteTransport) Deliver(w http.ResponseWriter, r *http.Request, p *Page) error {
	_, err := w.Write([]byte("PAGE"))
	return err
}
func (s stubRouteTransport) Routes(p *Page) map[string]http.Handler {
	return map[string]http.Handler{
		"/_regions/x": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*s.hit = true
			w.Write([]byte("ROUTE"))
		}),
	}
}

func TestServeMountsPageAndProviderRoutes(t *testing.T) {
	hit := false
	mux := http.NewServeMux()
	Serve(mux, "/", NewPage(routeShell), stubRouteTransport{hit: &hit})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Body.String() != "PAGE" {
		t.Fatalf("page body = %q", rec.Body.String())
	}

	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/_regions/x", nil))
	if rec2.Body.String() != "ROUTE" || !hit {
		t.Fatalf("route body = %q hit = %v", rec2.Body.String(), hit)
	}
}

func TestServeWithNonProviderTransportMountsPageOnly(t *testing.T) {
	mux := http.NewServeMux()
	Serve(mux, "/", NewPage(routeShell), StreamHTML{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestServeNilTransportDefaultsToStreamHTML(t *testing.T) {
	mux := http.NewServeMux()
	Serve(mux, "/", NewPage(routeShell), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("nil-transport Serve status = %d", rec.Code)
	}
}
