package quicken

import (
	"errors"
	"testing"

	"goforge.dev/goplus/std/result"
)

type resultTestRegion struct {
	mountErr    error
	renderPanic bool
}

func (resultTestRegion) ID() string                                   { return "x" }
func (resultTestRegion) Skeleton(RenderContext) Tree                  { return Text("loading") }
func (r resultTestRegion) Mount(RenderContext, Params) (State, error) { return nil, r.mountErr }
func (resultTestRegion) HandleEvent(RenderContext, string, Payload, State) (State, error) {
	return nil, nil
}
func (r resultTestRegion) Render(State) Tree {
	if r.renderPanic {
		panic("boom")
	}
	return Text("ok")
}

func TestRenderLiveFirstReturnsTypedMountFailure(t *testing.T) {
	lr := resultTestRegion{mountErr: errors.New("no mount")}
	got := renderLiveFirst(lr, RenderContext{})
	errResult, ok := got.(result.Err[mountedRegion, RenderFailure])
	if !ok {
		t.Fatalf("result = %T, want Err", got)
	}
	if _, ok := errResult.Err.(MountFailed); !ok {
		t.Fatalf("failure = %T, want MountFailed", errResult.Err)
	}
}

func TestSafeRenderReturnsTypedPanic(t *testing.T) {
	lr := resultTestRegion{renderPanic: true}
	got := safeRender(lr, nil)
	errResult, ok := got.(result.Err[Tree, RenderFailure])
	if !ok {
		t.Fatalf("result = %T, want Err", got)
	}
	if _, ok := errResult.Err.(RenderPanicked); !ok {
		t.Fatalf("failure = %T, want RenderPanicked", errResult.Err)
	}
}
