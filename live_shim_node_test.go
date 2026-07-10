package quicken

import (
	"os/exec"
	"testing"
)

// TestLiveShimBehaviorViaNode runs the live client functions against a
// hand-rolled DOM under node, asserting patch morphing and event delegation.
// It skips when node is not installed; node is a test-time tool.
func TestLiveShimBehaviorViaNode(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed; skipping live shim behavior test")
	}
	out, err := exec.Command("node", "client/live_dom_test.js").CombinedOutput()
	if err != nil {
		t.Fatalf("live shim node test failed: %v\n%s", err, out)
	}
}
