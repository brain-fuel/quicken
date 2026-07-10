package quicken

import (
	"context"
	"net/http"
)

// RenderContext carries request scope into a region's render. Later phases add a
// resume token and per-connection data; the type is the stable seam.
type RenderContext struct {
	Ctx context.Context
	R   *http.Request
}

// State is a region's opaque per-connection value. Deferred (non-live) regions
// do not use it; it is the seam the live phase builds on.
type State any

// Params are the mount parameters for a live region.
type Params map[string]any

// Payload is the client-supplied data on a live event.
type Payload map[string]any

// Region is a deferred-render unit: a cheap Skeleton shown immediately and an
// expensive Render produced afterward. Both return a Tree.
type Region interface {
	ID() string
	Skeleton(ctx RenderContext) Tree
	Render(ctx RenderContext) Tree
}

// LiveRegion adds server-held state and event handling. It is defined here so
// the core seam is stable; the live transport that drives it arrives in a
// later phase. A plain Region is the degenerate live region with no events.
type LiveRegion interface {
	ID() string
	Skeleton(ctx RenderContext) Tree
	Mount(ctx RenderContext, params Params) (State, error)
	HandleEvent(ctx RenderContext, name string, payload Payload, s State) (State, error)
	Render(s State) Tree
}

// RegionFunc builds a Region from an id and two render functions, so callers
// need not declare a type for simple regions.
func RegionFunc(id string, skeleton, render func(RenderContext) Tree) Region {
	return funcRegion{id: id, skeleton: skeleton, render: render}
}

type funcRegion struct {
	id       string
	skeleton func(RenderContext) Tree
	render   func(RenderContext) Tree
}

func (f funcRegion) ID() string                    { return f.id }
func (f funcRegion) Skeleton(c RenderContext) Tree { return f.skeleton(c) }
func (f funcRegion) Render(c RenderContext) Tree   { return f.render(c) }
