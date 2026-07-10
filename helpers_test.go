package quicken

import (
	"html/template"
	"strings"
	"testing"
)

func TestHelpersEmitMarkers(t *testing.T) {
	fm := Helpers()
	tmpl := template.Must(template.New("p").Funcs(fm).Parse(
		`<head>{{quickenHead}}</head><body>{{lazy "cards"}}{{live "counter"}}</body>`))
	out, err := RenderMarkup(tmpl, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<!--quicken head-->`,
		`<!--quicken lazy cards-->`,
		`<!--quicken live counter-->`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered %q missing %q", out, want)
		}
	}
}

func TestHelpersRoundTripThroughFromMarkup(t *testing.T) {
	// A template rendered with Helpers feeds FromMarkup and yields a working
	// page: the markers become slots.
	tmpl := template.Must(template.New("p").Funcs(Helpers()).Parse(
		`<body>{{lazy "cards"}}</body>`))
	markup, err := RenderMarkup(tmpl, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := FromMarkup(markup).Add(RegionFunc("cards",
		func(RenderContext) Tree { return Text("sk") },
		func(RenderContext) Tree { return Text("real") }))
	got := renderShell(p)
	if !strings.Contains(got, `id="q-slot-cards"`) || !strings.Contains(got, `data-q-pending`) {
		t.Fatalf("shell = %q", got)
	}
}

func TestHelpersLazyPanicsOnInvalidID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on invalid id")
		}
	}()
	fm := Helpers()
	lazy := fm["lazy"].(func(string) template.HTML)
	lazy("bad/id")
}

func TestRenderMarkupPropagatesExecError(t *testing.T) {
	// A template that fails to execute returns the error. Note: with nil data,
	// html/template silently renders a missing field as "<no value>" instead of
	// erroring (the dot is an untyped nil interface, which text/template treats
	// as a no-op field lookup); a concrete data value without the field is
	// required to force the "can't evaluate field" error.
	tmpl := template.Must(template.New("p").Parse(`{{.Missing.Field}}`))
	if _, err := RenderMarkup(tmpl, struct{}{}); err == nil {
		t.Fatal("expected execute error")
	}
}
