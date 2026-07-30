package quicken

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFrameContextExposesRequestScope pins the seam a dynamic route needs.
// A Region reaches request scope through Skeleton/Render's RenderContext;
// without Frame.Context the Shell had no equivalent, so the markup around
// the slots could not vary by request and a single Page could not host a
// dynamic path.
func TestFrameContextExposesRequestScope(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ot/PSA/8?rank-min=20000", nil)
	var seen RenderContext
	p := NewPage(func(f *Frame) template.HTML {
		seen = f.Context()
		return template.HTML("<!doctype html><html><body></body></html>")
	})

	f := &Frame{page: p, ctx: RenderContext{Ctx: req.Context(), R: req}}
	_ = p.shell(f)

	if seen.R == nil {
		t.Fatal("Frame.Context lost the request")
	}
	if got := seen.R.URL.Path; got != "/ot/PSA/8" {
		t.Fatalf("path = %q, want /ot/PSA/8", got)
	}
	if got := seen.R.URL.Query().Get("rank-min"); got != "20000" {
		t.Fatalf("query lost: rank-min = %q", got)
	}
}

func TestFrameContextZeroValueIsUsable(t *testing.T) {
	p := NewPage(func(*Frame) template.HTML { return template.HTML("") })
	f := &Frame{page: p}
	if c := f.Context(); c.R != nil || c.Ctx != nil {
		t.Fatalf("zero frame context = %+v, want empty", c)
	}
}
