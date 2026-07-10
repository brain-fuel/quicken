package quicken

import (
	"io"
	"net/http"
	"strings"
)

// ClientFetch delivers the shell with skeletons immediately and lets the
// client fetch each region from its own endpoint after load. The initial
// response contains no region content, so it is small and fast; regions fill
// in via the shim's fetch path and can be prefetched on intent. Use Serve so
// the per-region endpoints (Routes) are mounted alongside the page.
type ClientFetch struct{}

// Deliver implements Transport. It writes the shell with skeletons plus a JSON
// manifest telling the client which regions to fetch and from where. It does
// not render any region.
func (ClientFetch) Deliver(w http.ResponseWriter, r *http.Request, p *Page) error {
	ctx := RenderContext{Ctx: r.Context(), R: r}
	frame := &Frame{page: p, ctx: ctx}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	doc := string(p.shell(frame))
	head, tail := splitBody(doc)

	if _, err := io.WriteString(w, head); err != nil {
		return err
	}
	if _, err := io.WriteString(w, clientFetchManifest(p)); err != nil {
		return err
	}
	_, err := io.WriteString(w, tail)
	return err
}

// Routes implements RouteProvider: one endpoint per region that renders just
// that region, plus a 404 guard at the region prefix. The exact per-region
// routes are more specific than the guard, so known ids resolve and unknown
// ids return Not Found rather than falling through to a catch-all page handler.
func (ClientFetch) Routes(p *Page) map[string]http.Handler {
	routes := make(map[string]http.Handler, len(p.order)+1)
	for _, id := range p.order {
		region := p.regions[id]
		routes[regionPath(p.name, id)] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := RenderContext{Ctx: r.Context(), R: r}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, renderRegion(region, ctx))
		})
	}
	routes[regionPrefix(p.name)] = http.NotFoundHandler()
	return routes
}

// clientFetchManifest is the JSON script the shim reads to learn which regions
// to fetch. The page name and region ids are validated to [A-Za-z0-9_-]+ on
// Named/Add, so they are safe inside the JSON and require no escaping beyond
// the quoting jsStringLiteral provides.
func clientFetchManifest(p *Page) string {
	var b strings.Builder
	b.WriteString(`<script type="application/json" data-q-manifest>{"base":`)
	b.WriteString(jsStringLiteral(regionBase))
	b.WriteString(`,"page":`)
	b.WriteString(jsStringLiteral(p.name))
	b.WriteString(`,"ids":[`)
	for i, id := range p.order {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(jsStringLiteral(id))
	}
	b.WriteString(`]}</script>`)
	return b.String()
}
