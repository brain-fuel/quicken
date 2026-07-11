package quicken

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goforge.dev/cadence"
)

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

	tags := map[string]fillTag{
		"a": {Strategy: "eager", Trigger: "onload"},
		"b": {Strategy: "deferred", Trigger: "onvisible"},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	if err := renderFloor(rec, req, p, func(id string) fillTag { return tags[id] }); err != nil {
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

	s, err := strategyFor(p, nil, "a", cadence.RequestContext{})
	if err != nil {
		t.Fatalf("strategyFor: %v", err)
	}
	if s.Kind != cadence.Deferred || s.Where != cadence.Server || s.On != cadence.OnLoad {
		t.Errorf("plain region default = %+v, want Deferred{Server,OnLoad}", s)
	}
	if tg := tagOf(s); tg.Strategy != "deferred" || tg.Trigger != "onload" {
		t.Errorf("tagOf = %+v", tg)
	}
}

func TestTagOfAllBranches(t *testing.T) {
	cases := []struct {
		name string
		in   cadence.Strategy
		want fillTag
	}{
		{"eager", cadence.Strategy{Kind: cadence.Eager}, fillTag{Strategy: "eager", Trigger: "onload"}},
		{"live", cadence.Strategy{Kind: cadence.Live}, fillTag{Strategy: "live", Trigger: ""}},
		{"deferred-onload", cadence.Strategy{Kind: cadence.Deferred, Where: cadence.Server, On: cadence.OnLoad}, fillTag{Strategy: "deferred", Trigger: "onload"}},
		{"deferred-onvisible", cadence.Strategy{Kind: cadence.Deferred, Where: cadence.Server, On: cadence.OnVisible}, fillTag{Strategy: "deferred", Trigger: "onvisible"}},
		{"deferred-onhover", cadence.Strategy{Kind: cadence.Deferred, Where: cadence.Server, On: cadence.OnHover}, fillTag{Strategy: "deferred", Trigger: "onhover"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tagOf(tc.in); got != tc.want {
				t.Errorf("%s: tagOf(%+v) = %+v, want %+v", tc.name, tc.in, got, tc.want)
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
		"hot":  {Kind: cadence.Eager},
		"cold": {Kind: cadence.Deferred, Where: cadence.Server, On: cadence.OnVisible},
	})
	mux := http.NewServeMux()
	serveComposite(mux, "/", p, pol)

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

func TestStrategyForPolicyOverrideAndClientReject(t *testing.T) {
	p := NewPage(func(f *Frame) template.HTML { return "" })
	p.Add(textRegion("a", "AAA")).Add(textRegion("b", "BBB"))

	pol := cadence.Fixed(map[string]cadence.Strategy{
		"a": {Kind: cadence.Eager},
		"b": {Kind: cadence.Deferred, Where: cadence.Client, On: cadence.OnLoad},
	})
	s, err := strategyFor(p, pol, "a", cadence.RequestContext{})
	if err != nil || s.Kind != cadence.Eager {
		t.Fatalf("override a: %+v err=%v", s, err)
	}
	if _, err := strategyFor(p, pol, "b", cadence.RequestContext{}); err == nil {
		t.Error("Deferred{Client} must be rejected in SP2")
	}
}

func TestServeCompositeDegradesClientStrategy(t *testing.T) {
	p := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<html><body>" + string(f.Slot("c")) + "</body></html>")
	})
	p.Add(textRegion("c", "CLIENT-CONTENT"))

	pol := cadence.Fixed(map[string]cadence.Strategy{
		"c": {Kind: cadence.Deferred, Where: cadence.Client, On: cadence.OnLoad},
	})
	mux := http.NewServeMux()
	serveComposite(mux, "/", p, pol)

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
