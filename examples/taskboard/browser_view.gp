package taskboard

import (
	"fmt"

	"goforge.dev/cadence/style"
	browser "goforge.dev/quicken/web/browser"
)

func BrowserView(model Model) browser.Node[Msg] {
	tasks := VisibleTasks(model)
	rows := make([]browser.Node[Msg], 0, len(tasks))
	for _, task := range tasks {
		current := task
		checkbox := browser.Element[Msg]("button", browser.Text[Msg](checkLabel(current))).
			WithKey(fmt.Sprintf("toggle-%d", current.ID)).
			WithAttribute("type", "button").
			On(
				fmt.Sprintf("toggle-%d", current.ID),
				browser.EventClick(),
				func(browser.EventData) Msg { return ToggleRequested(current.ID) },
			)
		remove := browser.Element[Msg]("button", browser.Text[Msg]("Delete")).
			WithKey(fmt.Sprintf("delete-%d", current.ID)).
			WithAttribute("type", "button").
			On(
				fmt.Sprintf("delete-%d", current.ID),
				browser.EventClick(),
				func(browser.EventData) Msg { return DeleteRequested(current.ID) },
			)
		rows = append(rows, browser.Element[Msg]("li", checkbox, remove).
			WithKey(fmt.Sprintf("task-%d", current.ID)))
	}
	input := browser.Element[Msg]("input").
		WithKey("new-task").
		WithAttribute("name", "draft").
		WithAttribute("aria-label", "New task").
		WithProperty("value", model.Draft).
		On("new-task-input", browser.EventInput(), func(event browser.EventData) Msg {
			return DraftChanged(event.Value)
		})
	form := browser.Element[Msg](
		"form",
		input,
		browser.Element[Msg]("button", browser.Text[Msg]("Add task")).
			WithAttribute("type", "submit"),
	).
		WithKey("add-form").
		WithAttribute("method", "post").
		WithAttribute("action", "/taskboard").
		On("add-submit", browser.EventSubmit(), func(browser.EventData) Msg {
			return AddRequested()
		})
	status := "All changes saved."
	if model.Saving {
		status = "Saving changes..."
	}
	if model.Error != "" {
		status = model.Error
	}
	return browser.Element[Msg](
		"main",
		browser.Element[Msg]("h1", browser.Text[Msg]("Cadence Task Board")),
		form,
		browser.Element[Msg](
			"nav",
			filterButton("All", "all", FilterAll()),
			filterButton("Active", "active", FilterActive()),
			filterButton("Completed", "completed", FilterCompleted()),
		).WithAttribute("aria-label", "Task filters"),
		browser.Element[Msg]("ul", rows...).WithKey("tasks"),
		browser.Element[Msg]("p", browser.Text[Msg](status)).
			WithAttribute("role", "status"),
	).
		WithKey("taskboard").
		WithStyle(style.Default().WithPadding(style.All(1)))
}

func filterButton(label, key string, filter Filter) browser.Node[Msg] {
	return browser.Element[Msg]("button", browser.Text[Msg](label)).
		WithKey("filter-" + key).
		WithAttribute("type", "button").
		On("filter-"+key, browser.EventClick(), func(browser.EventData) Msg {
			return FilterRequested(filter)
		})
}

func checkLabel(task Task) string {
	if task.Done {
		return "[x] " + task.Title
	}
	return "[ ] " + task.Title
}
