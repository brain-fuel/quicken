package quicken

import "net/http"

// Transport delivers a page's shell and regions to the client. StreamHTML is
// the default; ClientFetch and later live transports sit behind this same
// interface.
type Transport interface {
	Deliver(w http.ResponseWriter, r *http.Request, p *Page) error
}

// RouteProvider is an optional Transport capability: extra routes a transport
// needs mounted alongside the page, such as the per-region fetch endpoints of
// ClientFetch. Serve mounts these automatically.
type RouteProvider interface {
	Routes(p *Page) map[string]http.Handler
}

// Serve mounts a page and its transport's routes on mux: the page handler at
// path, plus any routes the transport provides via RouteProvider. Call Mount
// once, separately, to serve the client shim. A nil transport uses StreamHTML.
func Serve(mux *http.ServeMux, path string, p *Page, t Transport) {
	if t == nil {
		t = StreamHTML{}
	}
	mux.Handle(path, p.Handler(t))
	if rp, ok := t.(RouteProvider); ok {
		for route, h := range rp.Routes(p) {
			mux.Handle(route, h)
		}
	}
}

// regionBase is the URL prefix under which per-region fetch endpoints live.
const regionBase = "/_regions"

// regionPath is the endpoint for one region: /_regions/<id> for an unnamed
// page, or /_regions/<name>/<id> when the page is named.
func regionPath(name, id string) string {
	if name == "" {
		return regionBase + "/" + id
	}
	return regionBase + "/" + name + "/" + id
}
