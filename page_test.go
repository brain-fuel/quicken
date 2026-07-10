package quicken

import (
	"html/template"
	"strings"
	"testing"
)

func emptyShell(*Frame) template.HTML { return "" }

func teaser(id string) Region {
	return RegionFunc(id,
		func(RenderContext) Tree { return Text("skel-" + id) },
		func(RenderContext) Tree { return Text("real-" + id) },
	)
}

func TestAddRejectsDuplicateID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate id")
		}
	}()
	NewPage(emptyShell).Add(teaser("a")).Add(teaser("a"))
}

func TestAddRejectsInvalidID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on invalid id")
		}
	}()
	NewPage(emptyShell).Add(teaser("bad id!"))
}

func TestAddRejectsEmptyID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on empty id")
		}
	}()
	NewPage(emptyShell).Add(teaser(""))
}

func TestFrameSlotRendersSkeletonWrapper(t *testing.T) {
	p := NewPage(emptyShell).Add(teaser("cards"))
	f := &Frame{page: p, ctx: RenderContext{}}
	got := string(f.Slot("cards"))
	for _, want := range []string{`id="q-slot-cards"`, "data-q-slot", "data-q-pending", "skel-cards"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Slot = %q, missing %q", got, want)
		}
	}
}

func TestFrameSlotMissingRegion(t *testing.T) {
	p := NewPage(emptyShell)
	f := &Frame{page: p, ctx: RenderContext{}}
	got := string(f.Slot("nope"))
	if !strings.Contains(got, "data-q-missing") {
		t.Fatalf("Slot for missing region = %q, want data-q-missing", got)
	}
}

func TestFrameSlotMissingIDEscaped(t *testing.T) {
	p := NewPage(emptyShell)
	f := &Frame{page: p, ctx: RenderContext{}}
	got := string(f.Slot(`x" onmouseover="alert(1)`))
	if strings.Contains(got, `onmouseover="alert(1)"`) {
		t.Fatalf("Slot did not escape a hostile id: %q", got)
	}
	if !strings.Contains(got, "data-q-missing") {
		t.Fatalf("Slot lost its missing marker: %q", got)
	}
}

func TestFrameHeadReferencesScript(t *testing.T) {
	f := &Frame{page: NewPage(emptyShell), ctx: RenderContext{}}
	got := string(f.Head())
	if !strings.Contains(got, ScriptPath) || !strings.Contains(got, "<script") {
		t.Fatalf("Head = %q, want script tag with %q", got, ScriptPath)
	}
}

type stubLive struct{ id string }

func (s stubLive) ID() string                  { return s.id }
func (s stubLive) Skeleton(RenderContext) Tree { return Text("sk") }
func (s stubLive) Mount(RenderContext, Params) (State, error) {
	return 0, nil
}
func (s stubLive) HandleEvent(RenderContext, string, Payload, State) (State, error) {
	return 0, nil
}
func (s stubLive) Render(State) Tree { return Text("r") }

func TestPageAddLiveRegistersAndOrders(t *testing.T) {
	p := NewPage(func(*Frame) template.HTML { return "" }).
		AddLive(stubLive{id: "a"}).
		AddLive(stubLive{id: "b"})
	got := p.liveRegions()
	if len(got) != 2 || got[0].ID() != "a" || got[1].ID() != "b" {
		t.Fatalf("liveRegions order = %v", got)
	}
}

func TestPageAddLiveRejectsDuplicateAcrossKinds(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on id already used by a deferred region")
		}
	}()
	NewPage(func(*Frame) template.HTML { return "" }).
		Add(RegionFunc("dup", func(RenderContext) Tree { return Text("s") },
			func(RenderContext) Tree { return Text("r") })).
		AddLive(stubLive{id: "dup"})
}

func TestPageAddLiveRejectsInvalidID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on invalid id")
		}
	}()
	NewPage(emptyShell).AddLive(stubLive{id: "bad id!"})
}

func TestPageAddLiveRejectsDuplicateLiveID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on id already used by a live region")
		}
	}()
	NewPage(emptyShell).AddLive(stubLive{id: "dup"}).AddLive(stubLive{id: "dup"})
}

func TestAddRejectsDuplicateAcrossKinds(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on id already used by a live region")
		}
	}()
	NewPage(emptyShell).AddLive(stubLive{id: "dup"}).Add(teaser("dup"))
}

func TestFrameSlotRendersLiveSkeletonWrapper(t *testing.T) {
	p := NewPage(emptyShell).AddLive(stubLive{id: "cards"})
	f := &Frame{page: p, ctx: RenderContext{}}
	got := string(f.Slot("cards"))
	for _, want := range []string{`id="q-slot-cards"`, "data-q-slot", "data-q-live", "data-q-pending", "sk"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Slot = %q, missing %q", got, want)
		}
	}
}

func TestSplitBody(t *testing.T) {
	head, tail := splitBody("<html><body>x</body></html>")
	if head != "<html><body>x" {
		t.Fatalf("head = %q", head)
	}
	if tail != "</body></html>" {
		t.Fatalf("tail = %q", tail)
	}
	h2, t2 := splitBody("no body tag")
	if h2 != "no body tag" || t2 != "" {
		t.Fatalf("no-body split = %q / %q", h2, t2)
	}
}
