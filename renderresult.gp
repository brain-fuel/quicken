package quicken

import (
	"fmt"

	"goforge.dev/goplus/std/result"
)

// RenderFailure makes mount errors and recovered render panics explicit.
//
//goplus:derive off
type RenderFailure enum {
	MountFailed(err error)
	RenderPanicked
}

func (f RenderFailure) Error() string {
	match f {
	case MountFailed(err):
		return fmt.Sprintf("mount failed: %v", err)
	case RenderPanicked():
		return "render panicked"
	}
	return "render failed"
}

type mountedRegion struct {
	tree  Tree
	state State
}

func renderLiveFirst(lr LiveRegion, ctx RenderContext) (out result.Result[mountedRegion, RenderFailure]) {
	defer func() {
		if recover() != nil {
			out = result.Err[mountedRegion, RenderFailure](RenderPanicked())
		}
	}()
	s, err := lr.Mount(ctx, nil)
	if err != nil {
		return result.Err[mountedRegion, RenderFailure](MountFailed(err))
	}
	return result.Ok[mountedRegion, RenderFailure](mountedRegion{tree: lr.Render(s), state: s})
}

func safeRender(lr LiveRegion, s State) (out result.Result[Tree, RenderFailure]) {
	defer func() {
		if recover() != nil {
			out = result.Err[Tree, RenderFailure](RenderPanicked())
		}
	}()
	return result.Ok[Tree, RenderFailure](lr.Render(s))
}
