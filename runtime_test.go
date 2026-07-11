package quicken

import (
	"html/template"
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
