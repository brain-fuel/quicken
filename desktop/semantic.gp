package desktop

import (
	"goforge.dev/cadence/sel"
	native "goforge.dev/quicken/native"
)

// SELView applies desktop density and control sizing to a shared semantic tree.
func SELView[Model any, Msg any](
	render func(Model) sel.Element[Msg],
	palette native.Palette,
) native.View[Model, Msg] {
	return native.SELViewWithIdiom(render, palette, native.DesktopIdiom())
}
