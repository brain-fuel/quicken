package main

import (
	"log"

	"gioui.org/layout"
	taskboard "goforge.dev/cadence-taskboard"
	mobile "goforge.dev/quicken/mobile"
	native "goforge.dev/quicken/native"
)

func main() {
	semantic := native.SELView(taskboard.SemanticView, native.DefaultPalette())
	view := func(
		gtx layout.Context,
		model taskboard.Model,
		ui *native.UI[taskboard.Msg],
		_ mobile.Environment,
	) layout.Dimensions {
		return semantic(gtx, model, ui)
	}
	_, err := mobile.Run(mobile.Application[taskboard.Model, taskboard.Msg]{
		ID: "dev.goforge.taskboard",
		Logic: taskboard.Logic(taskboard.InitialModel(), taskboard.LocalSave),
		View: view,
	})
	if err != nil {
		log.Fatal(err)
	}
}
