package quicken

import (
	"os/exec"
	"testing"
)

// TestShimSwapBehaviorViaNode runs the swap shim against a minimal hand-rolled
// DOM under node, so the JavaScript behavior is asserted, not just that the
// file is served. It skips when node is not installed; node is a test-time
// tool, not a module dependency.
func TestShimSwapBehaviorViaNode(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed; skipping JS shim behavior test")
	}
	out, err := exec.Command("node", "client/shim_dom_test.js").CombinedOutput()
	if err != nil {
		t.Fatalf("shim node test failed: %v\n%s", err, out)
	}
}
