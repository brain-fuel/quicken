package native

import (
	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"goforge.dev/cadence/program"
)

type View[State any, Msg any] func(
	layout.Context,
	State,
	*UI[Msg],
) layout.Dimensions

type WindowOptions struct {
	Title  string
	Width  unit.Dp
	Height unit.Dp
}

func NewWindow(options WindowOptions) *app.Window {
	window := &app.Window{}
	values := make([]app.Option, 0, 2)
	if options.Title != "" {
		values = append(values, app.Title(options.Title))
	}
	if options.Width > 0 && options.Height > 0 {
		values = append(values, app.Size(options.Width, options.Height))
	}
	if len(values) > 0 {
		window.Option(values...)
	}
	return window
}

// RunWindow owns one Gio window event loop. The caller must arrange app.Main
// on the process main goroutine as required by Gio.
func RunWindow[State any, Msg any](
	window *app.Window,
	logic program.Logic[State, Msg],
	view View[State, Msg],
	handler EffectHandler[Msg],
	theme *material.Theme,
) (State, error) {
	executor := NewExecutor(handler)
	runtime := program.NewRuntime(logic, executor)
	ui := NewUI[Msg](theme)
	runtime.SetObserver(func(State) { window.Invalidate() })
	runtime.Start()
	var operations op.Ops
	for {
		switch event := window.Event().(type) {
		case app.DestroyEvent:
			return runtime.Snapshot(), event.Err
		case app.FrameEvent:
			context := app.NewContext(&operations, event)
			ui.BeginFrame()
			view(context, runtime.Snapshot(), ui)
			event.Frame(context.Ops)
			for _, message := range ui.EndFrame() {
				runtime.Dispatch(message)
			}
		}
	}
}

func Main() { app.Main() }
