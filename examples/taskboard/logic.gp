package taskboard

import (
	"fmt"
	"strings"
	"time"

	"goforge.dev/cadence/program"
	browser "goforge.dev/quicken/web/browser"
)

type SaveEffect func(Model) program.Cmd[Msg]

type SaveRequest struct {
	Incidents []Incident `json:"incidents"`
}

type SaveResponse struct {
	Incidents []Incident `json:"incidents"`
}

func Logic(initial Model, save SaveEffect) program.Logic[Model, Msg] {
	return program.Logic[Model, Msg]{
		Init: func() program.Step[Model, Msg] {
			return program.Step[Model, Msg]{Model: initial, Command: program.None[Msg]()}
		},
		Update: func(model Model, message Msg) program.Step[Model, Msg] {
			return Update(model, message, save)
		},
	}
}

func Update(model Model, message Msg, save SaveEffect) program.Step[Model, Msg] {
	command := program.None[Msg]()
	match message {
	case IncidentDraftChanged(value): model.IncidentDraft = value; model.Error = ""
	case SummaryDraftChanged(value): model.SummaryDraft = value
	case LocationDraftChanged(value): model.LocationDraft = value
	case OwnerDraftChanged(value): model.OwnerDraft = value
	case IncidentSubmitted():
		title := strings.TrimSpace(model.IncidentDraft)
		if title == "" {
			model.Error = "An incident title is required."
		} else {
			incident := Incident{
				ID: model.NextIncidentID, Title: title,
				Summary: strings.TrimSpace(model.SummaryDraft),
				Location: strings.TrimSpace(model.LocationDraft),
				Owner: strings.TrimSpace(model.OwnerDraft),
				Severity: SeverityMedium, Status: StatusOpen,
				Timeline: []Activity{{ID: model.NextActivityID, At: nowLabel(), Kind: "opened", Body: "Incident opened."}},
			}
			model.Incidents = append(cloneIncidents(model.Incidents), incident)
			model.SelectedID = incident.ID
			model.NextIncidentID++
			model.NextActivityID++
			model.IncidentDraft, model.SummaryDraft = "", ""
			model.LocationDraft, model.OwnerDraft = "", ""
			model = dirty(model)
			command = save(model)
		}
	case IncidentSelected(id): model.SelectedID = id; model.Error = ""
	case SeverityChanged(id, severity):
		model.Incidents = updateIncident(model.Incidents, id, func(incident Incident) Incident {
			incident.Severity = severity
			return appendActivity(incident, &model, "severity", "Severity changed to "+string(severity)+".")
		})
		model = dirty(model); command = save(model)
	case StatusAdvanced(id):
		model.Incidents = updateIncident(model.Incidents, id, func(incident Incident) Incident {
			incident.Status = nextStatus(incident.Status)
			return appendActivity(incident, &model, "status", "Status changed to "+string(incident.Status)+".")
		})
		model = dirty(model); command = save(model)
	case TaskDraftChanged(value): model.TaskDraft = value
	case TaskAdded():
		title := strings.TrimSpace(model.TaskDraft)
		if title == "" || model.SelectedID == 0 {
			model.Error = "Select an incident and enter a task."
		} else {
			taskID := model.NextTaskID
			model.Incidents = updateIncident(model.Incidents, model.SelectedID, func(incident Incident) Incident {
				incident.Tasks = append(append([]Task(nil), incident.Tasks...), Task{ID: taskID, Title: title})
				return appendActivity(incident, &model, "task", "Task added: "+title)
			})
			model.NextTaskID++; model.TaskDraft = ""; model = dirty(model); command = save(model)
		}
	case TaskToggled(taskID):
		model.Incidents = updateIncident(model.Incidents, model.SelectedID, func(incident Incident) Incident {
			tasks := append([]Task(nil), incident.Tasks...)
			for i := range tasks { if tasks[i].ID == taskID { tasks[i].Done = !tasks[i].Done } }
			incident.Tasks = tasks
			return incident
		})
		model = dirty(model); command = save(model)
	case NoteDraftChanged(value): model.NoteDraft = value
	case NoteAdded():
		body := strings.TrimSpace(model.NoteDraft)
		if body == "" || model.SelectedID == 0 {
			model.Error = "Select an incident and enter a timeline note."
		} else {
			model.Incidents = updateIncident(model.Incidents, model.SelectedID, func(incident Incident) Incident {
				return appendActivity(incident, &model, "note", body)
			})
			model.NoteDraft = ""; model = dirty(model); command = save(model)
		}
	case SearchChanged(value): model.Query = value
	case FilterRequested(filter): model.Filter = filter
	case ConnectivityChanged(online):
		model.Online = online
		if online {
			model.Sync = SyncPending()
		} else {
			model.Error = "Offline: changes remain queued on this device."
		}
	case SyncRequested():
		if !model.Online {
			model.Error = "Cannot synchronize while offline."
		} else {
			model.Sync = SyncRunning(); model.Saving = true; command = save(model)
		}
	case SaveSucceeded(incidents):
		model.Incidents = cloneIncidents(incidents); model.Saving = false
		model.Sync = SyncIdle(); model.Error = ""
	case SaveFailed(message):
		model.Saving = false; model.Sync = SyncPending(); model.Error = message
	case ConflictDetected(conflict):
		model.Saving = false; model.Sync = SyncConflicted(conflict)
		model.Error = "A remote edit conflicts with this device."
	case KeepLocalRequested():
		conflict, ok := syncConflict(model.Sync)
		if ok {
			model.Incidents = replaceIncident(model.Incidents, conflict.Local)
			model.Sync = SyncPending(); model = dirty(model); command = save(model)
		}
	case AcceptRemoteRequested():
		conflict, ok := syncConflict(model.Sync)
		if ok {
			model.Incidents = replaceIncident(model.Incidents, conflict.Remote)
			model.Sync = SyncIdle(); model.Saving = false; model.Error = ""
		}
	}
	return program.Step[Model, Msg]{Model: model, Command: command}
}

func LocalSave(model Model) program.Cmd[Msg] {
	return program.After(75*time.Millisecond, program.Emit[Msg](SaveSucceeded(cloneIncidents(model.Incidents))))
}

func RemoteSave(model Model) program.Cmd[Msg] {
	requestID := fmt.Sprintf("save-%d", model.MutationRevision)
	return browser.MustRemote(
		requestID, "forgeflow.save", SaveRequest{Incidents: cloneIncidents(model.Incidents)},
		func(response SaveResponse) Msg { return SaveSucceeded(cloneIncidents(response.Incidents)) },
		func(failure browser.PublicError) Msg { return SaveFailed(failure.Message) },
	)
}

func VisibleIncidents(model Model) []Incident {
	out := make([]Incident, 0, len(model.Incidents))
	for _, incident := range model.Incidents {
		matchesQuery := model.Query == "" || containsFold(incident.Title, model.Query) ||
			containsFold(incident.Summary, model.Query) || containsFold(incident.Owner, model.Query) ||
			containsFold(incident.Location, model.Query)
		matchesFilter := FilterFold(model.Filter, FilterCases[bool]{
			FilterAll: func() bool { return true },
			FilterActive: func() bool { return incident.Status != StatusResolved },
			FilterCritical: func() bool { return incident.Severity == SeverityCritical },
			FilterResolved: func() bool { return incident.Status == StatusResolved },
		})
		if matchesQuery && matchesFilter { out = append(out, incident) }
	}
	return out
}

func dirty(model Model) Model {
	model.MutationRevision++
	model.Saving = true; model.Error = ""
	if model.Online { model.Sync = SyncRunning() } else { model.Sync = SyncPending() }
	return model
}

func updateIncident(incidents []Incident, id int, update func(Incident) Incident) []Incident {
	out := cloneIncidents(incidents)
	for i := range out { if out[i].ID == id { out[i] = update(out[i]) } }
	return out
}

func replaceIncident(incidents []Incident, replacement Incident) []Incident {
	return updateIncident(incidents, replacement.ID, func(Incident) Incident { return replacement })
}

func appendActivity(incident Incident, model *Model, kind, body string) Incident {
	incident.Timeline = append(append([]Activity(nil), incident.Timeline...), Activity{
		ID: model.NextActivityID, At: nowLabel(), Kind: kind, Body: body,
	})
	model.NextActivityID++
	return incident
}

func nextStatus(status IncidentStatus) IncidentStatus {
	switch status {
	case StatusOpen: return StatusMitigating
	case StatusMitigating: return StatusMonitoring
	case StatusMonitoring: return StatusResolved
	default: return StatusOpen
	}
}

func syncConflict(sync SyncState) (Conflict, bool) {
	var conflict Conflict
	found := false
	SyncStateFold(sync, SyncStateCases[bool]{
		SyncIdle: func() bool { return false }, SyncPending: func() bool { return false },
		SyncRunning: func() bool { return false },
		SyncConflicted: func(value Conflict) bool { conflict = value; found = true; return true },
	})
	return conflict, found
}
