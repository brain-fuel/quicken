package quicken

import (
	"html/template"
	"strings"

	"goforge.dev/cadence"
)

// Shell renders the full HTML document for a page. It places each region's
// skeleton with f.Slot(id) and includes the shim with f.Head(). It returns a
// complete document; the transport streams region fills in just before the
// closing body tag.
type Shell func(f *Frame) template.HTML

// PageRegion is one registered page capability. StaticRegion can render a
// floor value; StatefulRegion additionally mounts resumable state.
type PageRegion enum {
	StaticRegion(region Region)
	StatefulRegion(region LiveRegion)
}

// Page is a lazy page: a shell plus one ordered registry of region sums.
type Page struct {
	name    string
	shell   Shell
	regions map[string]PageRegion
	order   []string
}

// NewPage creates a page with the given shell.
func NewPage(shell Shell) *Page {
	return &Page{shell: shell, regions: map[string]PageRegion{}}
}

// Named sets the page's name, used to namespace its live endpoints (the
// WebSocket, poll, and event routes). The name must be empty or of the form
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
	p.regions[id] = StaticRegion(r)
	p.order = append(p.order, id)
	return p
}

// AddLive registers a live region. Live regions render in the order added. A
// duplicate id (in either the deferred or live set) or an id not of the form
// [A-Za-z0-9_-]+ panics, which is always a programming error.
func (p *Page) AddLive(r LiveRegion) *Page {
	id := r.ID()
	if !validID(id) {
		panic("quicken: invalid region id " + id)
	}
	if _, ok := p.regions[id]; ok {
		panic("quicken: duplicate region id " + id)
	}
	p.regions[id] = StatefulRegion(r)
	p.order = append(p.order, id)
	return p
}

func (p *Page) liveRegions() []LiveRegion {
	var out []LiveRegion
	for _, id := range p.order {
		match p.regions[id] {
		case StatefulRegion(r):
			out = append(out, r)
		case StaticRegion(_):
		}
	}
	return out
}

func (p *Page) liveIDs() []string {
	ids := make([]string, 0, len(p.order))
	for _, id := range p.order {
		match p.regions[id] {
		case StatefulRegion(_):
			ids = append(ids, id)
		case StaticRegion(_):
		}
	}
	return ids
}

func (p *Page) staticIDs() []string {
	ids := make([]string, 0, len(p.order))
	for _, id := range p.order {
		match p.regions[id] {
		case StaticRegion(_):
			ids = append(ids, id)
		case StatefulRegion(_):
		}
	}
	return ids
}

func (p *Page) staticRegion(id string) (Region, bool) {
	entry, ok := p.regions[id]
	if !ok {
		return nil, false
	}
	match entry {
	case StaticRegion(r):
		return r, true
	case StatefulRegion(_):
		return nil, false
	}
	return nil, false
}

func (p *Page) liveRegion(id string) (LiveRegion, bool) {
	entry, ok := p.regions[id]
	if !ok {
		return nil, false
	}
	match entry {
	case StatefulRegion(r):
		return r, true
	case StaticRegion(_):
		return nil, false
	}
	return nil, false
}

func (p *Page) regionKind(id string) cadence.RegionKind {
	match p.regions[id] {
	case StatefulRegion(_):
		return cadence.Stateful()
	case StaticRegion(_):
		return cadence.Plain()
	}
	return cadence.Plain()
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
	if entry, ok := f.page.regions[id]; ok {
		match entry {
		case StaticRegion(r):
			sk := r.Skeleton(f.ctx).HTML()
			return template.HTML(`<div id="q-slot-` + id + `" data-q-slot data-q-pending>` + sk + `</div>`)
		case StatefulRegion(r):
			sk := r.Skeleton(f.ctx).HTML()
			return template.HTML(`<div id="q-slot-` + id + `" data-q-slot data-q-live data-q-pending>` + sk + `</div>`)
		}
	}
	esc := template.HTMLEscapeString(id)
	return template.HTML(`<div id="q-slot-` + esc + `" data-q-slot data-q-missing></div>`)
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
