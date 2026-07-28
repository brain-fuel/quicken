package taskboard

import (
	"fmt"

	"goforge.dev/cadence/style"
	browser "goforge.dev/quicken/web/browser"
)

func BrowserView(model Model) browser.Node[Msg] {
	cards := make([]browser.Node[Msg], 0, len(model.Incidents))
	for _, incident := range VisibleIncidents(model) {
		current := incident
		cards = append(cards, browser.Element[Msg](
			"li",
			browser.Element[Msg]("button", browser.Text[Msg](
				fmt.Sprintf("%s · %s · %s", current.Severity, current.Status, current.Title),
			)).WithAttribute("type", "button").On(
				fmt.Sprintf("select-%d", current.ID), browser.EventClick(),
				func(browser.EventData) Msg { return IncidentSelected(current.ID) },
			),
			browser.Element[Msg]("small", browser.Text[Msg](current.Location+" · owner "+current.Owner)),
		).WithKey(fmt.Sprintf("incident-%d", current.ID)))
	}
	detail := browser.Element[Msg]("p", browser.Text[Msg]("Select an incident."))
	if incident, ok := selectedIncident(model); ok {
		tasks := make([]browser.Node[Msg], 0, len(incident.Tasks))
		for _, task := range incident.Tasks {
			current := task
			tasks = append(tasks, browser.Element[Msg]("li",
				browser.Element[Msg]("button", browser.Text[Msg](taskLabel(current))).
					WithAttribute("type", "button").
					On(fmt.Sprintf("task-%d", current.ID), browser.EventClick(),
						func(browser.EventData) Msg { return TaskToggled(current.ID) }),
			))
		}
		events := make([]browser.Node[Msg], 0, len(incident.Timeline))
		for _, event := range incident.Timeline {
			events = append(events, browser.Element[Msg]("li", browser.Text[Msg](event.At+" · "+event.Kind+" · "+event.Body)))
		}
		detail = browser.Element[Msg]("section",
			browser.Element[Msg]("h2", browser.Text[Msg](incident.Title)),
			browser.Element[Msg]("p", browser.Text[Msg](incident.Summary)),
			button("severity-low", "Low", SeverityChanged(incident.ID, SeverityLow)),
			button("severity-high", "High", SeverityChanged(incident.ID, SeverityHigh)),
			button("severity-critical", "Critical", SeverityChanged(incident.ID, SeverityCritical)),
			button("advance-status", "Advance status", StatusAdvanced(incident.ID)),
			browser.Element[Msg]("h3", browser.Text[Msg]("Response tasks")),
			browser.Element[Msg]("ul", tasks...),
			textInput("task-draft", model.TaskDraft, "New response task", func(value string) Msg { return TaskDraftChanged(value) }),
			button("task-add", "Add task", TaskAdded()),
			browser.Element[Msg]("h3", browser.Text[Msg]("Timeline")),
			browser.Element[Msg]("ol", events...),
			textInput("note-draft", model.NoteDraft, "Operational note", func(value string) Msg { return NoteDraftChanged(value) }),
			button("note-add", "Add note", NoteAdded()),
		)
	}
	return browser.Element[Msg](
		"main",
		browser.Element[Msg]("header",
			browser.Element[Msg]("h1", browser.Text[Msg]("ForgeFlow Operations")),
			browser.Element[Msg]("p", browser.Text[Msg](syncLabel(model))).WithAttribute("role", "status"),
		),
		textInput("search", model.Query, "Search incidents", func(value string) Msg { return SearchChanged(value) }),
		browser.Element[Msg]("nav",
			button("filter-all", "All", FilterRequested(FilterAll())),
			button("filter-active", "Active", FilterRequested(FilterActive())),
			button("filter-critical", "Critical", FilterRequested(FilterCritical())),
			button("filter-resolved", "Resolved", FilterRequested(FilterResolved())),
			button("sync", "Sync now", SyncRequested()),
		).WithAttribute("aria-label", "Incident filters"),
		browser.Element[Msg]("section",
			browser.Element[Msg]("aside", browser.Element[Msg]("h2", browser.Text[Msg]("Incidents")), browser.Element[Msg]("ul", cards...)),
			detail,
		),
		browser.Element[Msg]("section",
			browser.Element[Msg]("h2", browser.Text[Msg]("Open an incident")),
			textInput("incident-title", model.IncidentDraft, "Title", func(value string) Msg { return IncidentDraftChanged(value) }),
			textInput("incident-summary", model.SummaryDraft, "Summary", func(value string) Msg { return SummaryDraftChanged(value) }),
			textInput("incident-location", model.LocationDraft, "Location", func(value string) Msg { return LocationDraftChanged(value) }),
			textInput("incident-owner", model.OwnerDraft, "Owner", func(value string) Msg { return OwnerDraftChanged(value) }),
			button("incident-submit", "Open incident", IncidentSubmitted()),
		),
		browser.Element[Msg]("p", browser.Text[Msg](model.Error)).WithAttribute("role", "alert"),
	).WithKey("forgeflow").WithStyle(style.Default().WithPadding(style.All(1)))
}

func textInput(key, value, label string, onInput func(string) Msg) browser.Node[Msg] {
	return browser.Element[Msg]("input").WithKey(key).WithAttribute("aria-label", label).
		WithAttribute("placeholder", label).WithProperty("value", value).
		On(key+"-input", browser.EventInput(), func(event browser.EventData) Msg { return onInput(event.Value) })
}

func button(key, label string, message Msg) browser.Node[Msg] {
	return browser.Element[Msg]("button", browser.Text[Msg](label)).WithKey(key).
		WithAttribute("type", "button").
		On(key+"-click", browser.EventClick(), func(browser.EventData) Msg { return message })
}

func taskLabel(task Task) string {
	if task.Done { return "[x] "+task.Title }
	return "[ ] "+task.Title
}
