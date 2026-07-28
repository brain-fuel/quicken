# Quicken TUI

The Bubble Tea v2 interpreter for `goforge.dev/cadence`, with Bubbles and Lip
Gloss integration.

```go
_, err := bubbletea.Run(
	logic,
	bubbletea.SELView(semanticView, bubbletea.DefaultPalette()),
	mapNativeMessage,
	effectHandler,
)
```

The adapter lowers Cadence commands into `tea.Cmd`, renders semantic elements
with Lip Gloss, maps native terminal messages into the closed application
message type, and provides a Bubbles text-input bridge. Domain state remains in
the shared Cadence model.

MIT, Copyright (c) 2026 Goforge.
