package quicken

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScriptHandlerServesShim(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, ScriptPath, nil)
	rec := httptest.NewRecorder()
	ScriptHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("Content-Type = %q, want javascript", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "function swap") || !strings.Contains(body, "window.__quicken") {
		t.Fatalf("shim body missing swap: %q", body)
	}
}

func TestMountRegistersScriptRoute(t *testing.T) {
	mux := http.NewServeMux()
	Mount(mux)
	req := httptest.NewRequest(http.MethodGet, ScriptPath, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("mounted status = %d, want 200", rec.Code)
	}
}
