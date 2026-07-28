package quicken

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProgramHeadersPermitWasmWithoutUnsafeEval(t *testing.T) {
	response := httptest.NewRecorder()
	writeProgramHeaders(response)

	policy := response.Header().Get("Content-Security-Policy")
	if !strings.Contains(policy, "script-src 'self' 'wasm-unsafe-eval'") {
		t.Fatalf("CSP does not permit WebAssembly compilation: %q", policy)
	}
	if strings.Contains(policy, " 'unsafe-eval'") {
		t.Fatalf("CSP grants broad unsafe evaluation: %q", policy)
	}
}
