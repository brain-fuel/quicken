package quicken

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func demoPage() *Page {
	shell := func(f *Frame) template.HTML {
		return template.HTML(
			"<!doctype html><html><head>" + string(f.Head()) +
				"</head><body><main>" +
				string(f.Slot("alpha")) + string(f.Slot("beta")) +
				"</main></body></html>")
	}
	return NewPage(shell).
		Add(RegionFunc("alpha",
			func(RenderContext) Tree { return Text("<i>loading alpha</i>") },
			func(RenderContext) Tree { return Text("<p>ALPHA CONTENT</p>") })).
		Add(RegionFunc("beta",
			func(RenderContext) Tree { return Text("<i>loading beta</i>") },
			func(RenderContext) Tree { return Text("<p>BETA CONTENT</p>") }))
}

// defaultResolve tags every region with the kind-inferred default a nil
// Policy produces for a plain region (Deferred{Server, OnLoad}), so a test can
// drive renderFloor without constructing a cadence.Policy.
func defaultResolve(string) fillTag { return fillTag{Strategy: "deferred", Trigger: "onload"} }

func TestStreamDeliversShellSkeletonsFillsAndClose(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	if err := renderFloor(rec, req, demoPage(), defaultResolve); err != nil {
		t.Fatalf("renderFloor error: %v", err)
	}
	body := rec.Body.String()

	// Content-Type is HTML.
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q", ct)
	}
	// Shell skeletons are present in place, pending.
	for _, want := range []string{`id="q-slot-alpha"`, "data-q-pending", "loading alpha", `id="q-slot-beta"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing shell part %q", want)
		}
	}
	// Both regions' real content is present (JavaScript-off readability floor).
	for _, want := range []string{"ALPHA CONTENT", "BETA CONTENT"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing real content %q", want)
		}
	}
	// Fill blocks and reveal scripts are present for both regions.
	for _, want := range []string{`data-q-fill="alpha"`, `data-q-fill="beta"`, `__quicken.reveal("alpha")`, `__quicken.reveal("beta")`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing fill/swap %q", want)
		}
	}
	// Fills come before the closing body tag; document closes exactly once.
	if strings.Count(body, "</body></html>") != 1 {
		t.Fatalf("want exactly one document close, body=%q", body)
	}
	if strings.Index(body, `data-q-fill="alpha"`) > strings.Index(body, "</body></html>") {
		t.Fatal("fill block came after the closing body tag")
	}
}

func TestStreamRegionPanicBecomesErrorCard(t *testing.T) {
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<!doctype html><html><head>" + string(f.Head()) +
			"</head><body>" + string(f.Slot("boom")) + string(f.Slot("ok")) + "</body></html>")
	}).
		Add(RegionFunc("boom",
			func(RenderContext) Tree { return Text("sk") },
			func(RenderContext) Tree { panic("render exploded") })).
		Add(RegionFunc("ok",
			func(RenderContext) Tree { return Text("sk") },
			func(RenderContext) Tree { return Text("<p>OK CONTENT</p>") }))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	if err := renderFloor(rec, req, page, defaultResolve); err != nil {
		t.Fatalf("renderFloor error: %v", err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "data-q-error") {
		t.Fatalf("panicking region did not yield an error card: %q", body)
	}
	if !strings.Contains(body, "OK CONTENT") {
		t.Fatal("a sibling region was lost when another panicked")
	}
}

func TestStreamHonorsContextCancellation(t *testing.T) {
	block := make(chan struct{})
	page := NewPage(func(f *Frame) template.HTML {
		return template.HTML("<!doctype html><html><head>" + string(f.Head()) +
			"</head><body>" + string(f.Slot("slow")) + "</body></html>")
	}).
		Add(RegionFunc("slow",
			func(RenderContext) Tree { return Text("sk") },
			func(RenderContext) Tree {
				<-block
				return Text("<p>SLOW CONTENT</p>")
			}))

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	errCh := make(chan error, 1)
	go func() {
		errCh <- renderFloor(rec, req, page, defaultResolve)
	}()

	cancel()
	err := <-errCh
	close(block) // let the still-running render goroutine finish, no leak.

	if err != context.Canceled {
		t.Fatalf("Deliver error = %v, want context.Canceled", err)
	}
}
