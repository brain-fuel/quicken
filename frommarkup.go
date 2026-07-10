package quicken

import (
	"html/template"
	"strings"
)

// FromMarkup builds a Page from an HTML document carrying quicken markers:
// <!--quicken head--> for the client shim, and <!--quicken lazy id--> or
// <!--quicken live id--> for a region's slot. Register the regions with Add
// and AddLive; the slot each marker produces follows the registration. It
// panics on malformed markup or a duplicate region marker, which is always a
// programming error.
func FromMarkup(markup string) *Page {
	segs, err := parseMarkup(markup)
	if err != nil {
		panic(err.Error())
	}
	seen := map[string]bool{}
	for _, sg := range segs {
		if sg.kind == kindLazy || sg.kind == kindLive {
			if seen[sg.text] {
				panic("quicken: duplicate region marker " + sg.text)
			}
			seen[sg.text] = true
		}
	}
	return NewPage(func(f *Frame) template.HTML {
		var b strings.Builder
		for _, sg := range segs {
			switch sg.kind {
			case kindLiteral:
				b.WriteString(sg.text)
			case kindHead:
				b.WriteString(string(f.Head()))
			case kindLazy:
				b.WriteString(string(f.slotFor("lazy", sg.text)))
			case kindLive:
				b.WriteString(string(f.slotFor("live", sg.text)))
			}
		}
		return template.HTML(b.String())
	})
}

// slotFor renders a marker's slot when the registered region kind matches the
// marker kind, and a mismatch slot otherwise (unknown id, or a live marker on
// a deferred region and vice versa).
func (f *Frame) slotFor(kind, id string) template.HTML {
	_, isLazy := f.page.regions[id]
	_, isLive := f.page.live[id]
	if (kind == "lazy" && isLazy) || (kind == "live" && isLive) {
		return f.Slot(id)
	}
	esc := template.HTMLEscapeString(id)
	return template.HTML(`<div id="q-slot-` + esc + `" data-q-slot data-q-mismatch></div>`)
}
