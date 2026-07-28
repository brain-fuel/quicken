package taskboard

import (
	"fmt"

	"goforge.dev/cadence/sel"
	"goforge.dev/cadence/style"
)

func SemanticView(model Model) sel.Element[Msg] {
	tasks := VisibleTasks(model)
	rows := make([]sel.Element[Msg], 0, len(tasks))
	for _, task := range tasks {
		current := task
		row := sel.Row[Msg](
				sel.Checkbox[Msg](
				current.Done,
				current.Title,
				ToggleRequested(current.ID),
			).WithKey(fmt.Sprintf("toggle-%d", current.ID)),
				sel.Button[Msg](
				"Delete",
				DeleteRequested(current.ID),
			).WithKey(fmt.Sprintf("delete-%d", current.ID)),
		).WithKey(fmt.Sprintf("task-%d", current.ID))
		rows = append(rows, row)
	}
	status := "All changes saved."
	if model.Saving {
		status = "Saving changes..."
	}
	if model.Error != "" {
		status = model.Error
	}
	return sel.Column[Msg](
		sel.Heading[Msg](1, "Cadence Task Board").WithStyle(
			style.Default().
				WithForeground(style.ColorPrimary()).
				WithEmphasis(style.EmphasisStrong()),
		),
			sel.TextInput[Msg](
			model.Draft,
			"New task",
			func(value string) Msg { return DraftChanged(value) },
		).WithKey("new-task"),
			sel.Button[Msg]("Add task", AddRequested()).WithKey("add-task"),
			sel.Row[Msg](
				sel.Button[Msg]("All", FilterRequested(FilterAll())).WithKey("filter-all"),
				sel.Button[Msg]("Active", FilterRequested(FilterActive())).WithKey("filter-active"),
				sel.Button[Msg]("Completed", FilterRequested(FilterCompleted())).WithKey("filter-completed"),
			),
			sel.List[Msg](rows...),
		sel.Status[Msg](status).WithStyle(
			style.Default().WithForeground(style.ColorMuted()),
		),
	).WithKey("taskboard").WithStyle(
		style.Default().
			WithPadding(style.All(1)).
			WithBorder(style.BorderRounded()),
	)
}
