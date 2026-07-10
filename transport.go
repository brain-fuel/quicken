package quicken

import "net/http"

// Transport delivers a page's shell and regions to the client. StreamHTML is
// the default; later phases add client-fetch and live transports behind this
// same interface.
type Transport interface {
	Deliver(w http.ResponseWriter, r *http.Request, p *Page) error
}
