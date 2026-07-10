package quicken

import (
	"html/template"
	"net/http"
	"strings"
)

// Shell renders the full HTML document for a page. It places each region's
// skeleton with f.Slot(id) and includes the shim with f.Head(). It returns a
// complete document; the transport streams region fills in just before the
// closing body tag.
type Shell func(f *Frame) template.HTML

// Page is a lazy page: a shell plus its registered regions.
type Page struct {
	name    string
	shell   Shell
	regions map[string]Region
	order   []string
}

// NewPage creates a page with the given shell.
func NewPage(shell Shell) *Page {
	return &Page{shell: shell, regions: map[string]Region{}}
}

// Named sets the page's name, used to namespace its per-region fetch
// endpoints (see ClientFetch). The name must be empty or of the form
// [A-Za-z0-9_-]+; an invalid name panics. Returns the page for chaining.
func (p *Page) Named(name string) *Page {
	if name != "" && !validID(name) {
		panic("quicken: invalid page name " + name)
	}
	p.name = name
	return p
}

// Add registers a region. Regions render in the order added. Adding a region
// whose id is a duplicate or is not of the form [A-Za-z0-9_-]+ panics, which
// is always a programming error.
func (p *Page) Add(r Region) *Page {
	id := r.ID()
	if !validID(id) {
		panic("quicken: invalid region id " + id)
	}
	if _, ok := p.regions[id]; ok {
		panic("quicken: duplicate region id " + id)
	}
	p.regions[id] = r
	p.order = append(p.order, id)
	return p
}

// Handler returns an http.Handler that serves the page with the given
// transport. A nil transport uses StreamHTML.
func (p *Page) Handler(t Transport) http.Handler {
	if t == nil {
		t = StreamHTML{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = t.Deliver(w, r, p)
	})
}

// Frame is handed to a Shell so it can place region skeletons and the shim.
type Frame struct {
	page *Page
	ctx  RenderContext
}

// Head returns the markup for the document head: the shim script tag.
func (f *Frame) Head() template.HTML {
	return template.HTML(`<script src="` + ScriptPath + `"></script>`)
}

// Slot returns the placeholder for the region with the given id: a slot
// element carrying the region's skeleton, which the transport later fills.
func (f *Frame) Slot(id string) template.HTML {
	r, ok := f.page.regions[id]
	if !ok {
		esc := template.HTMLEscapeString(id)
		return template.HTML(`<div id="q-slot-` + esc + `" data-q-slot data-q-missing></div>`)
	}
	sk := r.Skeleton(f.ctx).HTML()
	return template.HTML(`<div id="q-slot-` + id + `" data-q-slot data-q-pending>` + sk + `</div>`)
}

// splitBody splits a complete document at the last closing body tag so the
// transport can stream fills between the body content and the closing tags.
// With no closing body tag the whole document is head and the tail is empty.
func splitBody(doc string) (head, tail string) {
	i := strings.LastIndex(doc, "</body>")
	if i < 0 {
		return doc, ""
	}
	return doc[:i], doc[i:]
}

func validID(id string) bool {
	if id == "" {
		return false
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}
