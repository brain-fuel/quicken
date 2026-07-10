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
