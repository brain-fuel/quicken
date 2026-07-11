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

// nonDegradingInterpreter is the negative witness: unlike floorInterpreter,
// it does NOT collapse every strategy to the region's full render. For Eager
// it serves Render, but for every other strategy it serves Skeleton instead
// of degrading to Render under no-JS. This exists to prove
// TestFloorInterpreterFallbackHolds is not vacuous: an interpreter whose
// output genuinely varies by strategy must fail cadence.FallbackHolds.
type nonDegradingInterpreter struct{}

func (nonDegradingInterpreter) Serve(r cadence.Region, s cadence.Strategy, _ cadence.RequestContext) (cadence.Tree, error) {
	if s.Kind == cadence.Eager {
		return r.Render(cadence.RenderContext{}), nil
	}
	return r.Skeleton(cadence.RenderContext{}), nil
}

// TestFloorConformanceIsNotVacuous proves that cadence.FallbackHolds actually
// exercises strategy-dependent behavior rather than trivially passing. Using
// textRegion("x", "REAL"), whose skeleton ("skeleton-x") differs from its
// render ("REAL"), a non-degrading interpreter must fail the law: it serves
// Skeleton for non-Eager strategies under no-JS instead of degrading to
// Render. If FallbackHolds ever returned true here, the law-check would be
// vacuous — it would not actually be comparing strategy-dependent output.
func TestFloorConformanceIsNotVacuous(t *testing.T) {
	if cadence.FallbackHolds(nonDegradingInterpreter{}, textRegion("x", "REAL")) {
		t.Fatal("nonDegradingInterpreter must fail the fallback law; FallbackHolds returning true means the law-check is vacuous and is not exercising strategy-dependent behavior")
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
