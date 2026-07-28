// Package dom interprets Cadence browser trees against a real DOM under
// js/wasm. Non-browser builds retain the same API and return ErrUnavailable.
package dom

import (
	"goforge.dev/cadence/program"
	browser "goforge.dev/quicken/web/browser"
)

type Options[Model any, Msg any] struct {
	Manifest browser.Manifest
	Logic    program.Logic[Model, Msg]
	View     program.View[Model, Msg, browser.Node[Msg]]
	Executor program.Executor[Msg]
}

type Runtime[Model any, Msg any] struct {
	core    *program.Runtime[Model, Msg]
	unmount func()
	err     func() error
	setSink func(program.Dispatch[Msg])
}

func (r *Runtime[Model, Msg]) Dispatch(msg Msg) {
	r.core.Dispatch(msg)
}

func (r *Runtime[Model, Msg]) Model() Model {
	return r.core.Snapshot()
}

func (r *Runtime[Model, Msg]) Err() error {
	if r.err == nil {
		return nil
	}
	return r.err()
}

func (r *Runtime[Model, Msg]) Unmount() {
	if r.unmount != nil {
		r.unmount()
	}
}

// SetMessageSink redirects decoded DOM messages. Live server-owned programs
// use it to send messages to Quicken instead of applying Update locally.
func (r *Runtime[Model, Msg]) SetMessageSink(sink program.Dispatch[Msg]) {
	if r.setSink != nil {
		r.setSink(sink)
	}
}

func (r *Runtime[Model, Msg]) Synchronize(model Model) {
	r.core.Synchronize(model)
}
