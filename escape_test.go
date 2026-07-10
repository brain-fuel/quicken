package quicken

import (
	"html/template"
	"strings"
	"testing"
)

func TestEscape(t *testing.T) {
	in := `<a href="x">&`
	got := Escape(in)
	want := template.HTMLEscapeString(in)
	if got != want {
		t.Fatalf("Escape(%q) = %q, want %q", in, got, want)
	}
	if !strings.Contains(got, "&lt;") {
		t.Fatalf("Escape(%q) = %q, want it to contain &lt;", in, got)
	}
	if !strings.Contains(got, "&amp;") {
		t.Fatalf("Escape(%q) = %q, want it to contain &amp;", in, got)
	}
	if strings.Contains(got, "<a") {
		t.Fatalf("Escape(%q) = %q, want no raw <a", in, got)
	}
}
