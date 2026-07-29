package browser

import (
	"strings"
	"testing"
)

func TestRenderHTMLPreservesEmptyTextNodeForHydration(t *testing.T) {
	root := Element[string]("p", Text[string](""))
	rendered, err := RenderHTML(root, RenderOptions{ProgramID: "test", MountID: "root"})
	if err != nil {
		t.Fatal(err)
	}
	marker := "<!--" + EmptyTextMarker + "-->"
	if !strings.Contains(rendered, marker) {
		t.Fatalf("empty text marker missing from %q", rendered)
	}
}
