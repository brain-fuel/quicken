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

func TestServeNilPolicyMountsPage(t *testing.T) {
	mux := http.NewServeMux()
	Serve(mux, "/", NewPage(routeShell), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("nil-policy Serve status = %d", rec.Code)
	}
}
