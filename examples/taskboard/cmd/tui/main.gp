package main

import (
	"log"

	bubbletea "goforge.dev/quicken/tui"
	taskboard "goforge.dev/cadence-taskboard"
)

func main() {
	_, err := bubbletea.Run(
		taskboard.Logic(taskboard.InitialModel(), taskboard.LocalSave),
		bubbletea.SELView(taskboard.SemanticView, bubbletea.DefaultPalette()),
		nil,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}
}
