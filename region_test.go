package quicken

import "testing"

// Compile-time proof that the func-backed region satisfies Region.
var _ Region = funcRegion{}

func TestRegionFunc(t *testing.T) {
	r := RegionFunc("teaser",
		func(Context) Tree { return Text("loading") },
		func(Context) Tree { return Text("done") },
	)
	if r.ID() != "teaser" {
		t.Fatalf("ID = %q", r.ID())
	}
	if got := r.Skeleton(Context{}).HTML(); got != "loading" {
		t.Fatalf("Skeleton = %q", got)
	}
	if got := r.Render(Context{}).HTML(); got != "done" {
		t.Fatalf("Render = %q", got)
	}
}
