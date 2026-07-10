package quicken

import "testing"

func TestTextHTML(t *testing.T) {
	if got := Text("<p>hi</p>").HTML(); got != "<p>hi</p>" {
		t.Fatalf("HTML = %q", got)
	}
}

func TestSlotsHTML(t *testing.T) {
	tr := Slots([]string{"a", "b", "c"}, []string{"X", "Y"})
	if got := tr.HTML(); got != "aXbYc" {
		t.Fatalf("HTML = %q", got)
	}
}

func TestSlotsPanicsOnShapeMismatch(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on len mismatch")
		}
	}()
	Slots([]string{"a", "b"}, []string{"X", "Y"})
}

func TestDiffNoChange(t *testing.T) {
	a := Slots([]string{"s0", "s1"}, []string{"d"})
	b := Slots([]string{"s0", "s1"}, []string{"d"})
	changed, full := a.Diff(b)
	if full {
		t.Fatal("fullReplace = true, want false")
	}
	if len(changed) != 0 {
		t.Fatalf("changed = %v, want empty", changed)
	}
}

func TestDiffSlotChanged(t *testing.T) {
	a := Slots([]string{"s0", "s1", "s2"}, []string{"new", "same"})
	b := Slots([]string{"s0", "s1", "s2"}, []string{"old", "same"})
	changed, full := a.Diff(b)
	if full {
		t.Fatal("fullReplace = true, want false")
	}
	if len(changed) != 1 || changed[0] != "new" {
		t.Fatalf("changed = %v, want {0:new}", changed)
	}
}

func TestDiffFullReplaceOnStaticShapeChange(t *testing.T) {
	a := Slots([]string{"x", "y"}, []string{"d"})
	b := Slots([]string{"p", "q"}, []string{"d"})
	changed, full := a.Diff(b)
	if !full {
		t.Fatal("fullReplace = false, want true")
	}
	if changed != nil {
		t.Fatalf("changed = %v, want nil", changed)
	}
}
