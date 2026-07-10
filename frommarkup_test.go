package quicken

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// renderShell renders a page's shell to a string for assertions, the way a
// transport would before streaming fills.
func renderShell(p *Page) string {
	return string(p.shell(&Frame{page: p, ctx: RenderContext{}}))
}

func TestFromMarkupSubstitutesLazySlot(t *testing.T) {
	p := FromMarkup(`<main><!--quicken lazy cards--></main>`).
		Add(RegionFunc("cards",
			func(RenderContext) Tree { return Text("sk") },
			func(RenderContext) Tree { return Text("real") }))
	got := renderShell(p)
	want := `<main><div id="q-slot-cards" data-q-slot data-q-pending>sk</div></main>`
	if got != want {
		t.Fatalf("shell = %q, want %q", got, want)
	}
}

func TestFromMarkupSubstitutesLiveSlot(t *testing.T) {
	p := FromMarkup(`<main><!--quicken live counter--></main>`).AddLive(stubLive{id: "counter"})
	got := renderShell(p)
	if !strings.Contains(got, `id="q-slot-counter"`) || !strings.Contains(got, `data-q-live`) {
		t.Fatalf("live shell = %q", got)
	}
}

func TestFromMarkupHeadEmitsShim(t *testing.T) {
	p := FromMarkup(`<head><!--quicken head--></head>`)
	got := renderShell(p)
	if !strings.Contains(got, ScriptPath) {
		t.Fatalf("head shell = %q, want the shim script path", got)
	}
}

func TestFromMarkupKindMismatchRendersMismatchSlot(t *testing.T) {
	// A live marker whose id is registered as a deferred region is an author
	// error: render a mismatch slot, not the deferred slot.
	p := FromMarkup(`<!--quicken live cards-->`).
		Add(RegionFunc("cards",
			func(RenderContext) Tree { return Text("sk") },
			func(RenderContext) Tree { return Text("real") }))
	got := renderShell(p)
	if !strings.Contains(got, `data-q-mismatch`) {
		t.Fatalf("mismatch shell = %q, want data-q-mismatch", got)
	}
}

func TestFromMarkupUnknownIdRendersMismatchSlot(t *testing.T) {
	p := FromMarkup(`<!--quicken lazy ghost-->`)
	got := renderShell(p)
	if !strings.Contains(got, `data-q-mismatch`) {
		t.Fatalf("unknown-id shell = %q", got)
	}
}

func TestFromMarkupPanicsOnDuplicateMarker(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate marker id")
		}
	}()
	FromMarkup(`<!--quicken lazy dup--><!--quicken lazy dup-->`)
}

func TestFromMarkupPanicsOnDuplicateHead(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate head marker")
		}
	}()
	FromMarkup(`<head><!--quicken head--><!--quicken head--></head>`)
}

func TestFromMarkupPanicsOnMalformedMarkup(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on malformed markup")
		}
	}()
	FromMarkup(`<!--quicken lazy bad/id-->`)
}

func TestFromMarkupServesOverTransport(t *testing.T) {
	// A FromMarkup page is an ordinary Page: it delivers over StreamHTML like
	// any other, filling the region.
	p := FromMarkup(`<!doctype html><html><head><!--quicken head--></head><body><!--quicken lazy cards--></body></html>`).
		Add(RegionFunc("cards",
			func(RenderContext) Tree { return Text("sk") },
			func(RenderContext) Tree { return Text("<p>REAL</p>") }))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	if err := (StreamHTML{}).Deliver(rec, req, p); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "REAL") || !strings.Contains(body, `id="q-slot-cards"`) {
		t.Fatalf("delivered body = %q", body)
	}
}
