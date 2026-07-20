package quicken

import (
	"flag"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"goforge.dev/cadence"
)

// update regenerates golden files (testdata/*.golden.html) from the current
// output when set: `go test -run TestMixedStrategyGolden -update`. No golden
// convention existed in this package before this test; this is the minimal
// one, introduced here.
var update = flag.Bool("update", false, "update golden files")

func textRegion(id, body string) Region {
	return cadence.RegionFunc(id,
		func(cadence.RenderContext) cadence.Tree { return cadence.Text("skeleton-" + id) },
		func(cadence.RenderContext) cadence.Tree { return cadence.Text(body) },
	)
}

func TestRenderFloorTagsEachFill(t *testing.T) {
	p := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("a")) + string(f.Slot("b")) + "</body></html>")
	})
	p.Add(textRegion("a", "AAA")).Add(textRegion("b", "BBB"))

	tags := map[string]cadence.Interpretation{
		"a": cadence.Inline{},
		"b": cadence.AfterPaint{On: cadence.OnVisible{}},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	if err := renderFloor(rec, req, p, func(id string) cadence.Interpretation { return tags[id] }); err != nil {
		t.Fatalf("renderFloor: %v", err)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`data-q-fill="a" data-q-strategy="eager" data-q-trigger="onload"`,
		`>AAA</div>`,
		`data-q-fill="b" data-q-strategy="deferred" data-q-trigger="onvisible"`,
		`>BBB</div>`,
		`window.__quicken&&window.__quicken.reveal("a")`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("floor missing %q\n---\n%s", want, body)
		}
	}
	if strings.Index(body, `data-q-fill="a"`) > strings.Index(body, `data-q-fill="b"`) {
		t.Error("fills out of order")
	}
}

func TestStrategyForKindInferred(t *testing.T) {
	p := NewPage(func(f *Frame) template.HTML { return "" })
	p.Add(textRegion("a", "AAA"))

	s := strategyFor(p, nil, "a", cadence.RequestContext{})
	if _, ok := s.(cadence.AfterPaint); !ok {
		t.Errorf("plain region interpretation = %T, want AfterPaint", s)
	}
	if strategy, trigger := wireTag(s); strategy != "deferred" || trigger != "onload" {
		t.Errorf("wireTag = %q/%q", strategy, trigger)
	}
}

func TestTagOfAllBranches(t *testing.T) {
	cases := []struct {
		name     string
		in       cadence.Interpretation
		strategy string
		trigger  string
	}{
		{"eager", cadence.Inline{}, "eager", "onload"},
		{"live", cadence.LiveTransport{}, "live", ""},
		{"deferred-onload", cadence.AfterPaint{On: cadence.OnLoad{}}, "deferred", "onload"},
		{"deferred-onvisible", cadence.AfterPaint{On: cadence.OnVisible{}}, "deferred", "onvisible"},
		{"deferred-onhover", cadence.AfterPaint{On: cadence.OnHover{}}, "deferred", "onhover"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			strategy, trigger := wireTag(tc.in)
			if strategy != tc.strategy || trigger != tc.trigger {
				t.Errorf("%s: wireTag = %q/%q, want %q/%q", tc.name, strategy, trigger, tc.strategy, tc.trigger)
			}
		})
	}
}

func TestServeCompositeMixedStrategies(t *testing.T) {
	p := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><head>" + string(f.Head()) +
			"</head><body>" + string(f.Slot("hot")) + string(f.Slot("cold")) + "</body></html>")
	})
	p.Add(textRegion("hot", "HOT")).Add(textRegion("cold", "COLD"))

	pol := cadence.Fixed(map[string]cadence.Strategy{
		"hot":  cadence.Eager{},
		"cold": cadence.Deferred{Where: cadence.Server{}, On: cadence.OnVisible{}},
	})
	mux := http.NewServeMux()
	Serve(mux, "/", p, pol)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	body := rec.Body.String()

	if !strings.Contains(body, `id="q-slot-hot" data-q-slot`) {
		t.Error("missing hot slot")
	}
	if !strings.Contains(body, `data-q-fill="hot" data-q-strategy="eager" data-q-trigger="onload"`) {
		t.Errorf("hot fill wrong:\n%s", body)
	}
	if !strings.Contains(body, `data-q-fill="cold" data-q-strategy="deferred" data-q-trigger="onvisible"`) {
		t.Errorf("cold fill wrong:\n%s", body)
	}
	if !strings.Contains(body, `>HOT</div>`) || !strings.Contains(body, `>COLD</div>`) {
		t.Error("region content missing from floor (M1: server always renders every region)")
	}
}

// TestMixedStrategyGolden pins the exact bytes of a composite floor with all
// three SP2-supported strategy shapes side by side: Eager, Deferred{Server,
// OnVisible}, Deferred{Server,OnHover}. Regenerate with:
//
//	go test -run TestMixedStrategyGolden -update
func TestMixedStrategyGolden(t *testing.T) {
	p := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><head>" + string(f.Head()) + "</head><body>" +
			string(f.Slot("eager")) + string(f.Slot("vis")) + string(f.Slot("hov")) +
			"</body></html>")
	})
	p.Add(textRegion("eager", "EAGER-CONTENT")).
		Add(textRegion("vis", "VISIBLE-CONTENT")).
		Add(textRegion("hov", "HOVER-CONTENT"))

	pol := cadence.Fixed(map[string]cadence.Strategy{
		"eager": cadence.Eager{},
		"vis":   cadence.Deferred{Where: cadence.Server{}, On: cadence.OnVisible{}},
		"hov":   cadence.Deferred{Where: cadence.Server{}, On: cadence.OnHover{}},
	})
	mux := http.NewServeMux()
	Serve(mux, "/", p, pol)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	body := rec.Body.String()

	golden := filepath.Join("testdata", "mixed_floor.golden.html")
	if *update {
		if err := os.WriteFile(golden, []byte(body), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to generate it): %v", err)
	}
	if body != string(want) {
		t.Errorf("mixed-strategy floor mismatch (run with -update to regenerate):\ngot:\n%s\nwant:\n%s", body, want)
	}
}

func TestStrategyForPolicyOverrideAndClientReject(t *testing.T) {
	p := NewPage(func(f *Frame) template.HTML { return "" })
	p.Add(textRegion("a", "AAA")).Add(textRegion("b", "BBB"))

	pol := cadence.Fixed(map[string]cadence.Strategy{
		"a": cadence.Eager{},
		"b": cadence.Deferred{Where: cadence.Client{}, On: cadence.OnLoad{}},
	})
	s := strategyFor(p, pol, "a", cadence.RequestContext{})
	if _, ok := s.(cadence.Inline); !ok {
		t.Fatalf("override a: %T", s)
	}
	if _, ok := strategyFor(p, pol, "b", cadence.RequestContext{}).(cadence.ClientCompute); !ok {
		t.Error("Deferred{Client} must remain explicit as ClientCompute")
	}
}

func TestServeCompositeDegradesClientStrategy(t *testing.T) {
	p := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	})
	p.Add(textRegion("c", "CLIENT-CONTENT"))

	pol := cadence.Fixed(map[string]cadence.Strategy{
		"c": cadence.Deferred{Where: cadence.Client{}, On: cadence.OnLoad{}},
	})
	mux := http.NewServeMux()
	Serve(mux, "/", p, pol)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-q-fill="c" data-q-strategy="eager"`) {
		t.Errorf("region not degraded to eager:\n%s", body)
	}
	if !strings.Contains(body, "CLIENT-CONTENT") {
		t.Errorf("full content missing from degraded floor:\n%s", body)
	}
}

// TestServeCompositeDegradesLiveOnPlainRegion is Finding 2's regression: a
// Policy that assigns Live to a region registered with Add (not AddLive) must
// not stick behind its skeleton. Left as "live", reveal (which no-ops on
// "live") and swapLiveSnapshots (which only walks p.liveOrder, and this
// region is in neither p.live nor p.liveOrder) would never fill the slot with
// scripting on, even though the floor already has the real content.
func TestServeCompositeDegradesLiveOnPlainRegion(t *testing.T) {
	p := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("p")) + "</body></html>")
	})
	p.Add(textRegion("p", "PLAIN-CONTENT"))

	pol := cadence.Uniform(cadence.Live{})
	mux := http.NewServeMux()
	Serve(mux, "/", p, pol)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-q-fill="p" data-q-strategy="eager"`) {
		t.Errorf("Live-on-plain-region not degraded to eager:\n%s", body)
	}
	if !strings.Contains(body, "PLAIN-CONTENT") {
		t.Errorf("full content missing from degraded floor:\n%s", body)
	}
}

func TestServeCompositeWithLiveRegion(t *testing.T) {
	p := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><head>" + string(f.Head()) + "</head><body>" +
			string(f.Slot("clock")) + "</body></html>")
	})
	p.AddLive(newTestClock("clock")) // LiveRegion whose first Render contains "TICK-0"

	mux := http.NewServeMux()
	Serve(mux, "/", p, nil) // nil policy: live region → Live

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	body := rec.Body.String()

	if !strings.Contains(body, `data-q-fill="clock" data-q-strategy="live"`) {
		t.Errorf("live fill missing/mislabeled:\n%s", body)
	}
	if !strings.Contains(body, "TICK-0") {
		t.Error("live region's first render must be in the floor (no-JS snapshot)")
	}
	if !strings.Contains(body, `data-q-live`) {
		t.Error("live manifest script missing")
	}
	// A hardcoded unknown token (the brief's literal "token=x") cannot tell
	// "route not mounted" apart from "route mounted, token rejected": once
	// mounted, pollHandler already 404s any token its store does not
	// recognize (see longpoll_test.go's "unknown token poll status" case,
	// asserting exactly that), so "x" always 404s when the route IS mounted
	// -- while an *unmounted* poll path instead falls through to the "/"
	// composite handler and returns 200, not 404. That makes the literal
	// == StatusNotFound check backwards. Use the real token this request's
	// own manifest just minted instead, so a non-404 response demonstrates
	// the route is mounted and wired to the very session the floor rendered.
	m := regexp.MustCompile(`"token":"([^"]+)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatal("could not extract live token from manifest")
	}
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest("GET", liveBasePath("")+"/poll?token="+m[1], nil))
	// A mounted pollHandler answers a valid token's first poll with a JSON
	// "first" message and Content-Type: application/json (see longpoll.go's
	// pollHandler). An unmounted poll path instead falls through to the "/"
	// composite handler, which also returns 200 but with Content-Type:
	// text/html (see runtime.go's renderFloor) -- so both "mounted" and
	// "unmounted" answer 200, and a bare non-404 check can't tell them
	// apart. Assert the JSON content type and first-message marker instead:
	// only the mounted poll handler produces them.
	if rec2.Code != http.StatusOK {
		t.Errorf("live poll route not mounted: status = %d, body:\n%s", rec2.Code, rec2.Body.String())
	}
	if ct := rec2.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("live poll route not mounted (fell through to HTML floor): Content-Type = %q, body:\n%s", ct, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), `"type":"first"`) {
		t.Errorf("live poll response missing first-message marker:\n%s", rec2.Body.String())
	}
}

// TestServeWithSessionStoreOptionSharesStoreAcrossMountAndRequest is
// Finding 1's regression: public Serve had no way to inject a SessionStore,
// so every deployment was stuck on the never-evicting process-wide
// defaultStore despite the docs instructing production users to supply a
// bounded one. This drives the public API end-to-end (no unexported
// LiveChannel construction) and proves the SAME injected store backs both
// the per-request session mint and the routes Serve mounted at Serve time:
// if those two paths built separate LiveChannel values, each would silently
// fall back to its own defaultStore and this test would fail even though
// nothing panics.
func TestServeWithSessionStoreOptionSharesStoreAcrossMountAndRequest(t *testing.T) {
	p := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><head>" + string(f.Head()) + "</head><body>" +
			string(f.Slot("clock")) + "</body></html>")
	})
	p.AddLive(newTestClock("clock"))

	custom := NewMemoryStore()
	mux := http.NewServeMux()
	Serve(mux, "/", p, nil, WithSessionStore(custom))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	body := rec.Body.String()

	m := regexp.MustCompile(`"token":"([^"]+)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatal("could not extract live token from manifest")
	}
	token := m[1]

	// The token minted into the served manifest must land in the injected
	// custom store, not the process-wide default.
	if _, ok := custom.Get(token); !ok {
		t.Fatal("session minted for the served token is missing from the injected custom store")
	}

	// Driving the mounted poll route with that token must resolve against
	// the SAME custom store: a split store (mount-time vs per-request) would
	// 404 here even though custom.Get above just succeeded.
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest("GET", liveBasePath("")+"/poll?token="+token, nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("poll route did not resolve the custom-store session: status = %d, body:\n%s", rec2.Code, rec2.Body.String())
	}
	if ct := rec2.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("poll route not mounted against custom store: Content-Type = %q", ct)
	}
	if !strings.Contains(rec2.Body.String(), `"type":"first"`) {
		t.Errorf("poll response missing first-message marker:\n%s", rec2.Body.String())
	}
}
