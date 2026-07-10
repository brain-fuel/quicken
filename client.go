package quicken

import (
	_ "embed"
	"net/http"
	"strings"
	"time"
)

//go:embed client/quicken.js
var clientJS string

// ScriptPath is where the shim is served and referenced from the shell head.
const ScriptPath = "/_quicken/quicken.js"

// ScriptHandler serves the embedded swap shim as JavaScript.
func ScriptHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		http.ServeContent(w, r, "quicken.js", time.Time{}, strings.NewReader(clientJS))
	})
}

// Mount registers the shim route on mux at ScriptPath.
func Mount(mux *http.ServeMux) {
	mux.Handle(ScriptPath, ScriptHandler())
}
