# Quicken Desktop

`goforge.dev/quicken/desktop` runs Cadence programs as native Windows, macOS,
and Linux applications. It owns desktop application/window lifecycle and uses
`goforge.dev/quicken/native` for Gio rendering.

```go
model, err := desktop.Run(desktop.Application[Model, Msg]{
	Logic: logic,
	View: native.SELView(view, native.DefaultPalette()),
	Window: native.WindowOptions{Title: "Tasks"},
})
```

Platform packaging remains a build concern; the model, update, commands, and
semantic view are shared with Quicken Web, TUI, and Mobile.

MIT, Copyright (c) 2026 Goforge.
