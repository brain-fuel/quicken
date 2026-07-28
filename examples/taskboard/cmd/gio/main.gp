package main

import (
	"log"

	"gioui.org/unit"
	cadencegio "goforge.dev/quicken/native"
	taskboard "goforge.dev/cadence-taskboard"
)

func main() {
	window := cadencegio.NewWindow(cadencegio.WindowOptions{
		Title: "ForgeFlow Operations",
		Width: unit.Dp(720),
		Height: unit.Dp(540),
	})
	go func() {
		_, err := cadencegio.RunWindow(
			window,
			taskboard.Logic(taskboard.InitialModel(), taskboard.LocalSave),
			cadencegio.SELView(taskboard.SemanticView, cadencegio.DefaultPalette()),
			nil,
			nil,
		)
		if err != nil {
			log.Print(err)
		}
	}()
	cadencegio.Main()
}
