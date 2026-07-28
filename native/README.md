# Quicken Native

The Gio interpreter for `goforge.dev/cadence`.

```go
window := native.NewWindow(native.WindowOptions{Title: "Tasks"})
go native.RunWindow(
	window,
	logic,
	native.SELView(semanticView, native.DefaultPalette()),
	effectHandler,
	nil,
)
native.Main()
```

The adapter owns keyed mechanical widget state, translates semantic elements
and styles into Gio layout/material operations, and dispatches widget messages
after each frame into Cadence's serialized update loop.

MIT, Copyright (c) 2026 Goforge.
