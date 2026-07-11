package quicken

import (
	"strings"
	"testing"
)

// TestLiveFirstRenderMatchesDeferredContent asserts that the live counter at
// its mount state renders the same bytes a deferred region would for the same
// content: the streamed floor stitches a deferred region's Render(ctx).HTML(),
// and a live region's first render is Render(mountState); for equivalent
// content the two must agree byte for byte.
func TestLiveFirstRenderMatchesDeferredContent(t *testing.T) {
	c := counter{id: "c"}
	st, _ := c.Mount(RenderContext{}, nil)
	liveHTML := c.Render(st).HTML()

	deferred := RegionFunc("c",
		func(RenderContext) Tree { return Text("sk") },
		func(RenderContext) Tree { return Slots([]string{`<b>`, `</b>`}, []string{"0"}) })
	deferredHTML := deferred.Render(RenderContext{}).HTML()

	if liveHTML != deferredHTML {
		t.Fatalf("live first render %q != deferred %q", liveHTML, deferredHTML)
	}
}

// TestRenderLiveHTMLStripsToPlainStitch asserts that the slot-addressed live
// HTML, with its <q-d> marker wrappers removed, equals the plain stitched
// HTML, so the live wire carries the same content the other transports
// render, just addressed for patching.
func TestRenderLiveHTMLStripsToPlainStitch(t *testing.T) {
	tr := Slots([]string{`<b>`, `</b>`}, []string{"7"})
	plain := tr.HTML()
	live := renderLiveHTML(tr)
	stripped := stripMarkers(live)
	if stripped != plain {
		t.Fatalf("stripped live %q != plain %q", stripped, plain)
	}
}

// stripMarkers removes the <q-d data-qi="N"> and </q-d> wrapper markers that
// renderLiveHTML adds around each dynamic slot, leaving the plain stitch.
func stripMarkers(s string) string {
	out := s
	for {
		i := strings.Index(out, `<q-d data-qi="`)
		if i < 0 {
			break
		}
		j := strings.Index(out[i:], `">`)
		out = out[:i] + out[i+j+2:]
	}
	out = strings.ReplaceAll(out, `</q-d>`, "")
	return out
}
