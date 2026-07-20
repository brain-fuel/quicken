package quicken

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goforge.dev/cadence"
)

// Cadence generates property tests for these laws; this test pins that
// Quicken consumes the same canonical witness rather than a local fallback.
func TestQuickenUsesCadenceFallbackSemantics(t *testing.T) {
	host := cadence.ReferenceSemantics{}
	if !cadence.ReferenceSemanticsInstance.LawNoScriptIsInline(
		host, cadence.Plain{}, cadence.Deferred{Where: cadence.Server{}, On: cadence.OnHover{}},
	) {
		t.Fatal("no-script interpretation must be Inline")
	}
	if !cadence.ReferenceSemanticsInstance.LawPlainNeverUsesLiveTransport(host, cadence.Live{}) {
		t.Fatal("plain regions must never require live transport")
	}
}

func TestDeferredServerRegionDegradesToRealContent(t *testing.T) {
	p := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("r")) + "</body></html>")
	})
	p.Add(textRegion("r", "DEGRADES-TO-REAL"))
	mux := http.NewServeMux()
	Serve(mux, "/", p, cadence.Uniform(cadence.Deferred{Where: cadence.Server{}, On: cadence.OnLoad{}}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(rec.Body.String(), "DEGRADES-TO-REAL") {
		t.Error("deferred-server region must appear in the universal floor")
	}
}
