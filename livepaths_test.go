package quicken

import (
	"html/template"
	"strings"
	"testing"
)

func livePage(name string, ids ...string) *Page {
	p := NewPage(func(*Frame) template.HTML { return template.HTML("") })
	if name != "" {
		p = p.Named(name)
	}
	for _, id := range ids {
		p.AddLive(stubLive{id: id})
	}
	return p
}

func TestLivePathsUnnamedPage(t *testing.T) {
	got := LivePaths(livePage("", "a"))
	want := []string{"/_live/ws", "/_live/poll", "/_live/event"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("LivePaths = %v, want %v", got, want)
	}
}

func TestLivePathsNamedPage(t *testing.T) {
	got := LivePaths(livePage("demo", "a"))
	want := []string{"/_live/demo/ws", "/_live/demo/poll", "/_live/demo/event"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("LivePaths = %v, want %v", got, want)
	}
}

func TestLivePathsNilWithoutLiveRegions(t *testing.T) {
	if got := LivePaths(livePage("")); got != nil {
		t.Fatalf("LivePaths on a page with no live regions = %v, want nil", got)
	}
	if got := LivePaths(nil); got != nil {
		t.Fatalf("LivePaths(nil) = %v, want nil", got)
	}
}

// TestLivePathsMatchWhatServeMounts is the point of exporting them: a host
// mounting these on its own mux must hit the same handlers Serve registers.
func TestLivePathsMatchWhatServeMounts(t *testing.T) {
	p := livePage("", "a")
	lc := LiveChannel{}
	routes := lc.liveRoutes(p)
	for _, path := range LivePaths(p) {
		if _, ok := routes[path]; !ok {
			t.Errorf("LivePaths returned %q, which Serve does not mount", path)
		}
	}
	if len(routes) != len(LivePaths(p)) {
		t.Errorf("Serve mounts %d routes, LivePaths returns %d", len(routes), len(LivePaths(p)))
	}
}
