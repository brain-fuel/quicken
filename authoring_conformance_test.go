package quicken

import (
	"html/template"
	"testing"
)

// TestAuthoringStylesProduceSameShell asserts that the func-registry shell, the
// FromMarkup adapter, and the template-helper adapter all produce the same
// shell HTML for the same region set, so an app can switch authoring styles
// without changing what the transports deliver.
func TestAuthoringStylesProduceSameShell(t *testing.T) {
	cards := func() Region {
		return RegionFunc("cards",
			func(RenderContext) Tree { return Text("sk") },
			func(RenderContext) Tree { return Text("real") })
	}

	// Style A: func-registry shell.
	shellFn := func(f *Frame) template.HTML {
		return template.HTML("<body>" + string(f.Slot("cards")) + "</body>")
	}
	pa := NewPage(shellFn).Add(cards())

	// Style B: FromMarkup.
	pb := FromMarkup("<body><!--quicken lazy cards--></body>").Add(cards())

	// Style C: template helpers -> markup -> FromMarkup.
	tmpl := template.Must(template.New("p").Funcs(Helpers()).Parse(`<body>{{lazy "cards"}}</body>`))
	markup, err := RenderMarkup(tmpl, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc := FromMarkup(markup).Add(cards())

	a, b, c := renderShell(pa), renderShell(pb), renderShell(pc)
	if a != b || b != c {
		t.Fatalf("shells differ:\n A=%q\n B=%q\n C=%q", a, b, c)
	}
}

// TestAuthoringStylesProduceSameShellWithHeadAndLive extends the same
// equivalence to a shell that carries a head marker and a live region
// alongside the lazy region, so all three authoring styles are proven
// interchangeable for head + lazy + live together, not just lazy alone.
func TestAuthoringStylesProduceSameShellWithHeadAndLive(t *testing.T) {
	cards := func() Region {
		return RegionFunc("cards",
			func(RenderContext) Tree { return Text("sk") },
			func(RenderContext) Tree { return Text("real") })
	}
	counter := func() LiveRegion { return stubLive{id: "counter"} }

	// Style A: func-registry shell.
	shellFn := func(f *Frame) template.HTML {
		return template.HTML("<head>" + string(f.Head()) + "</head><body>" +
			string(f.Slot("cards")) + string(f.Slot("counter")) + "</body>")
	}
	pa := NewPage(shellFn).Add(cards()).AddLive(counter())

	// Style B: FromMarkup.
	pb := FromMarkup(`<head><!--quicken head--></head><body><!--quicken lazy cards--><!--quicken live counter--></body>`).
		Add(cards()).AddLive(counter())

	// Style C: template helpers -> markup -> FromMarkup.
	tmpl := template.Must(template.New("p").Funcs(Helpers()).
		Parse(`<head>{{quickenHead}}</head><body>{{lazy "cards"}}{{live "counter"}}</body>`))
	markup, err := RenderMarkup(tmpl, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc := FromMarkup(markup).Add(cards()).AddLive(counter())

	a, b, c := renderShell(pa), renderShell(pb), renderShell(pc)
	if a != b || b != c {
		t.Fatalf("shells differ:\n A=%q\n B=%q\n C=%q", a, b, c)
	}
}
