package quicken

import (
	"os/exec"
	"testing"
)

// runNodeScript runs a client-side JS scenario file under node, skipping when
// node is not installed. node is a test-time tool, not a module dependency.
func runNodeScript(t *testing.T, script string) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed; skipping JS test")
	}
	out, err := exec.Command("node", script).CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", script, err, out)
	}
}

func TestShimFetchRuntimeViaNode(t *testing.T) {
	runNodeScript(t, "client/shim_fetch_test.js")
}

func TestShimPrefetchViaNode(t *testing.T) {
	runNodeScript(t, "client/shim_prefetch_test.js")
}
