package quicken

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goforge.dev/cadence"
)

// floorInterpreter is the SP2 conformance witness: it serves a region's full
// Render(ctx) for every strategy, ignoring the strategy entirely. That is the
// M1 invariant (the streamed floor always renders every region in full), so
// the fallback law holds for it by construction: there is nothing for NoJS to
// degrade, because there is no lighter-weight path to begin with.
type floorInterpreter struct{}

func (floorInterpreter) Serve(r cadence.Region, _ cadence.Strategy, _ cadence.RequestContext) (cadence.Tree, error) {
	return r.Render(cadence.RenderContext{}), nil
}

func TestFloorInterpreterFallbackHolds(t *testing.T) {
	if !cadence.FallbackHolds(floorInterpreter{}, textRegion("x", "REAL")) {
		t.Error("SP2 floor interpreter must satisfy the fallback law")
	}
}

// TestDeferredServerRegionDegradesToRealContent asserts the Deferred{Server}
// counterpart to TestServeCompositeDegradesClientStrategy: a region whose
// resolved strategy is Deferred{Server,OnLoad} still has its full, real
// content streamed into the no-JS floor (M1: the composite runtime always
// renders every region's Render(ctx) into the floor, regardless of the
// strategy tag attached to its fill).
func TestDeferredServerRegionDegradesToRealContent(t *testing.T) {
	p := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("r")) + "</body></html>")
	})
	p.Add(textRegion("r", "DEGRADES-TO-REAL"))
	mux := http.NewServeMux()
	Serve(mux, "/", p, cadence.Uniform(cadence.Strategy{Kind: cadence.Deferred, Where: cadence.Server, On: cadence.OnLoad}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(rec.Body.String(), "DEGRADES-TO-REAL") {
		t.Error("deferred-server region must appear in the no-JS floor")
	}
}
