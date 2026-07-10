package quicken

import (
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
			func(Context) Tree { return Text("<i>loading alpha</i>") },
			func(Context) Tree { return Text("<p>ALPHA CONTENT</p>") })).
		Add(RegionFunc("beta",
			func(Context) Tree { return Text("<i>loading beta</i>") },
			func(Context) Tree { return Text("<p>BETA CONTENT</p>") }))
}

func TestStreamDeliversShellSkeletonsFillsAndClose(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	if err := (StreamHTML{}).Deliver(rec, req, demoPage()); err != nil {
		t.Fatalf("Deliver error: %v", err)
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
	// Fill blocks and swap scripts are present for both regions.
	for _, want := range []string{`data-q-fill="alpha"`, `data-q-fill="beta"`, `__quicken.swap("alpha")`, `__quicken.swap("beta")`} {
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
			func(Context) Tree { return Text("sk") },
			func(Context) Tree { panic("render exploded") })).
		Add(RegionFunc("ok",
			func(Context) Tree { return Text("sk") },
			func(Context) Tree { return Text("<p>OK CONTENT</p>") }))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	if err := (StreamHTML{}).Deliver(rec, req, page); err != nil {
		t.Fatalf("Deliver error: %v", err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "data-q-error") {
		t.Fatalf("panicking region did not yield an error card: %q", body)
	}
	if !strings.Contains(body, "OK CONTENT") {
		t.Fatal("a sibling region was lost when another panicked")
	}
}
