package taskboard

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"goforge.dev/cadence/program"
)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type IncidentStatus string

const (
	StatusOpen       IncidentStatus = "open"
	StatusMitigating IncidentStatus = "mitigating"
	StatusMonitoring IncidentStatus = "monitoring"
	StatusResolved   IncidentStatus = "resolved"
)

type Task struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Assignee string `json:"assignee,omitempty"`
}

type Activity struct {
	ID   int    `json:"id"`
	At   string `json:"at"`
	Kind string `json:"kind"`
	Body string `json:"body"`
}

type Incident struct {
	ID       int            `json:"id"`
	Title    string         `json:"title"`
	Summary  string         `json:"summary"`
	Location string         `json:"location"`
	Owner    string         `json:"owner"`
	Severity Severity       `json:"severity"`
	Status   IncidentStatus `json:"status"`
	Tasks    []Task         `json:"tasks"`
	Timeline []Activity     `json:"timeline"`
}

type Conflict struct {
	IncidentID int      `json:"incident_id"`
	Local      Incident `json:"local"`
	Remote     Incident `json:"remote"`
}

type Filter enum {
	FilterAll
	FilterActive
	FilterCritical
	FilterResolved
}

type SyncState enum {
	SyncIdle
	SyncPending
	SyncRunning
	SyncConflicted(conflict Conflict)
}

type Model struct {
	Incidents    []Incident
	SelectedID   int
	IncidentDraft string
	SummaryDraft string
	LocationDraft string
	OwnerDraft   string
	TaskDraft    string
	NoteDraft    string
	Query        string
	Filter       Filter
	Sync         SyncState
	Online       bool
	Saving       bool
	Error        string
	NextIncidentID int
	NextTaskID   int
	NextActivityID int
	MutationRevision uint64
}

//goplus:derive gen
type Msg enum {
	IncidentDraftChanged(value string)
	SummaryDraftChanged(value string)
	LocationDraftChanged(value string)
	OwnerDraftChanged(value string)
	IncidentSubmitted
	IncidentSelected(id int)
	SeverityChanged(id int, severity Severity)
	StatusAdvanced(id int)
	TaskDraftChanged(value string)
	TaskAdded
	TaskToggled(taskID int)
	NoteDraftChanged(value string)
	NoteAdded
	SearchChanged(value string)
	FilterRequested(filter Filter)
	ConnectivityChanged(online bool)
	SyncRequested
	SaveSucceeded(incidents []Incident)
	SaveFailed(message string)
	ConflictDetected(conflict Conflict)
	KeepLocalRequested
	AcceptRemoteRequested
}

type Bootstrap struct {
	Initial Model
}

type modelWire struct {
	Incidents       []Incident `json:"incidents"`
	SelectedID      int        `json:"selected_id"`
	IncidentDraft   string     `json:"incident_draft"`
	SummaryDraft    string     `json:"summary_draft"`
	LocationDraft   string     `json:"location_draft"`
	OwnerDraft      string     `json:"owner_draft"`
	TaskDraft       string     `json:"task_draft"`
	NoteDraft       string     `json:"note_draft"`
	Query           string     `json:"query"`
	Filter          string     `json:"filter"`
	Sync            string     `json:"sync"`
	Conflict        *Conflict  `json:"conflict,omitempty"`
	Online          bool       `json:"online"`
	Saving          bool       `json:"saving"`
	Error           string     `json:"error"`
	NextIncidentID  int        `json:"next_incident_id"`
	NextTaskID      int        `json:"next_task_id"`
	NextActivityID  int        `json:"next_activity_id"`
	MutationRevision uint64    `json:"mutation_revision"`
}

type messageWire struct {
	Type      string     `json:"type"`
	Value     string     `json:"value,omitempty"`
	ID        int        `json:"id,omitempty"`
	TaskID    int        `json:"task_id,omitempty"`
	Severity  Severity   `json:"severity,omitempty"`
	Filter    string     `json:"filter,omitempty"`
	Online    bool       `json:"online,omitempty"`
	Incidents []Incident `json:"incidents,omitempty"`
	Message   string     `json:"message,omitempty"`
	Conflict  *Conflict  `json:"conflict,omitempty"`
}

func InitialModel() Model {
	return Model{
		Incidents: []Incident{
			{
				ID: 1, Title: "Warehouse cooling alarm",
				Summary: "Temperature is rising in cold storage zone B.",
				Location: "Portland warehouse", Owner: "Maya",
				Severity: SeverityHigh, Status: StatusMitigating,
				Tasks: []Task{
					{ID: 1, Title: "Inspect compressor telemetry", Done: true, Assignee: "Lee"},
					{ID: 2, Title: "Move temperature-sensitive inventory", Assignee: "Maya"},
				},
				Timeline: []Activity{
					{ID: 1, At: "09:10", Kind: "opened", Body: "Sensor C-14 crossed the alert threshold."},
				},
			},
			{
				ID: 2, Title: "Checkout API latency",
				Summary: "p95 latency exceeds the regional SLO.",
				Location: "us-west", Owner: "Noor",
				Severity: SeverityCritical, Status: StatusMonitoring,
				Tasks: []Task{{ID: 3, Title: "Compare database replica lag", Done: true, Assignee: "Noor"}},
				Timeline: []Activity{{ID: 2, At: "10:42", Kind: "update", Body: "Traffic shifted away from the degraded pool."}},
			},
		},
		SelectedID: 1, Filter: FilterAll(), Sync: SyncIdle(), Online: true,
		NextIncidentID: 3, NextTaskID: 4, NextActivityID: 3,
	}
}

func ModelCodec() program.Codec[Model] {
	return program.Codec[Model]{
		Encode: func(model Model) ([]byte, error) {
			syncName, conflict := encodeSync(model.Sync)
			return json.Marshal(modelWire{
				Incidents: cloneIncidents(model.Incidents), SelectedID: model.SelectedID,
				IncidentDraft: model.IncidentDraft, SummaryDraft: model.SummaryDraft,
				LocationDraft: model.LocationDraft, OwnerDraft: model.OwnerDraft,
				TaskDraft: model.TaskDraft, NoteDraft: model.NoteDraft, Query: model.Query,
				Filter: filterName(model.Filter), Sync: syncName, Conflict: conflict,
				Online: model.Online, Saving: model.Saving, Error: model.Error,
				NextIncidentID: model.NextIncidentID, NextTaskID: model.NextTaskID,
				NextActivityID: model.NextActivityID,
				MutationRevision: model.MutationRevision,
			})
		},
		Decode: func(data []byte) (Model, error) {
			var wire modelWire
			if err := json.Unmarshal(data, &wire); err != nil {
				return Model{}, err
			}
			filter, err := parseFilter(wire.Filter)
			if err != nil {
				return Model{}, err
			}
			sync, err := decodeSync(wire.Sync, wire.Conflict)
			if err != nil {
				return Model{}, err
			}
			return Model{
				Incidents: cloneIncidents(wire.Incidents), SelectedID: wire.SelectedID,
				IncidentDraft: wire.IncidentDraft, SummaryDraft: wire.SummaryDraft,
				LocationDraft: wire.LocationDraft, OwnerDraft: wire.OwnerDraft,
				TaskDraft: wire.TaskDraft, NoteDraft: wire.NoteDraft, Query: wire.Query,
				Filter: filter, Sync: sync, Online: wire.Online, Saving: wire.Saving,
				Error: wire.Error, NextIncidentID: wire.NextIncidentID,
				NextTaskID: wire.NextTaskID, NextActivityID: wire.NextActivityID,
				MutationRevision: wire.MutationRevision,
			}, nil
		},
	}
}

func MessageCodec() program.Codec[Msg] {
	return program.Codec[Msg]{Encode: encodeMessage, Decode: decodeMessage}
}

func encodeMessage(message Msg) ([]byte, error) {
	var wire messageWire
	match message {
	case IncidentDraftChanged(value): wire = messageWire{Type: "incident-draft", Value: value}
	case SummaryDraftChanged(value): wire = messageWire{Type: "summary-draft", Value: value}
	case LocationDraftChanged(value): wire = messageWire{Type: "location-draft", Value: value}
	case OwnerDraftChanged(value): wire = messageWire{Type: "owner-draft", Value: value}
	case IncidentSubmitted(): wire = messageWire{Type: "incident-submitted"}
	case IncidentSelected(id): wire = messageWire{Type: "incident-selected", ID: id}
	case SeverityChanged(id, severity): wire = messageWire{Type: "severity-changed", ID: id, Severity: severity}
	case StatusAdvanced(id): wire = messageWire{Type: "status-advanced", ID: id}
	case TaskDraftChanged(value): wire = messageWire{Type: "task-draft", Value: value}
	case TaskAdded(): wire = messageWire{Type: "task-added"}
	case TaskToggled(taskID): wire = messageWire{Type: "task-toggled", TaskID: taskID}
	case NoteDraftChanged(value): wire = messageWire{Type: "note-draft", Value: value}
	case NoteAdded(): wire = messageWire{Type: "note-added"}
	case SearchChanged(value): wire = messageWire{Type: "search-changed", Value: value}
	case FilterRequested(filter): wire = messageWire{Type: "filter", Filter: filterName(filter)}
	case ConnectivityChanged(online): wire = messageWire{Type: "connectivity", Online: online}
	case SyncRequested(): wire = messageWire{Type: "sync-requested"}
	case SaveSucceeded(incidents): wire = messageWire{Type: "save-succeeded", Incidents: cloneIncidents(incidents)}
	case SaveFailed(message): wire = messageWire{Type: "save-failed", Message: message}
	case ConflictDetected(conflict): wire = messageWire{Type: "conflict", Conflict: &conflict}
	case KeepLocalRequested(): wire = messageWire{Type: "keep-local"}
	case AcceptRemoteRequested(): wire = messageWire{Type: "accept-remote"}
	}
	return json.Marshal(wire)
}

func decodeMessage(data []byte) (Msg, error) {
	var wire messageWire
	if err := json.Unmarshal(data, &wire); err != nil { return nil, err }
	switch wire.Type {
	case "incident-draft": return IncidentDraftChanged(wire.Value), nil
	case "summary-draft": return SummaryDraftChanged(wire.Value), nil
	case "location-draft": return LocationDraftChanged(wire.Value), nil
	case "owner-draft": return OwnerDraftChanged(wire.Value), nil
	case "incident-submitted": return IncidentSubmitted(), nil
	case "incident-selected": return IncidentSelected(wire.ID), nil
	case "severity-changed": return SeverityChanged(wire.ID, wire.Severity), nil
	case "status-advanced": return StatusAdvanced(wire.ID), nil
	case "task-draft": return TaskDraftChanged(wire.Value), nil
	case "task-added": return TaskAdded(), nil
	case "task-toggled": return TaskToggled(wire.TaskID), nil
	case "note-draft": return NoteDraftChanged(wire.Value), nil
	case "note-added": return NoteAdded(), nil
	case "search-changed": return SearchChanged(wire.Value), nil
	case "filter":
		filter, err := parseFilter(wire.Filter)
		if err != nil { return nil, err }
		return FilterRequested(filter), nil
	case "connectivity": return ConnectivityChanged(wire.Online), nil
	case "sync-requested": return SyncRequested(), nil
	case "save-succeeded": return SaveSucceeded(cloneIncidents(wire.Incidents)), nil
	case "save-failed": return SaveFailed(wire.Message), nil
	case "conflict":
		if wire.Conflict == nil { return nil, fmt.Errorf("forgeflow: conflict payload missing") }
		return ConflictDetected(*wire.Conflict), nil
	case "keep-local": return KeepLocalRequested(), nil
	case "accept-remote": return AcceptRemoteRequested(), nil
	default: return nil, fmt.Errorf("forgeflow: unknown message %q", wire.Type)
	}
}

func filterName(filter Filter) string {
	return FilterFold(filter, FilterCases[string]{
		FilterAll: func() string { return "all" },
		FilterActive: func() string { return "active" },
		FilterCritical: func() string { return "critical" },
		FilterResolved: func() string { return "resolved" },
	})
}

func parseFilter(value string) (Filter, error) {
	switch value {
	case "", "all": return FilterAll(), nil
	case "active": return FilterActive(), nil
	case "critical": return FilterCritical(), nil
	case "resolved": return FilterResolved(), nil
	default: return nil, fmt.Errorf("forgeflow: invalid filter %q", value)
	}
}

func encodeSync(sync SyncState) (string, *Conflict) {
	return SyncStateFold(sync, SyncStateCases[struct{Name string; Conflict *Conflict}]{
		SyncIdle: func() struct{Name string; Conflict *Conflict} { return struct{Name string; Conflict *Conflict}{"idle", nil} },
		SyncPending: func() struct{Name string; Conflict *Conflict} { return struct{Name string; Conflict *Conflict}{"pending", nil} },
		SyncRunning: func() struct{Name string; Conflict *Conflict} { return struct{Name string; Conflict *Conflict}{"running", nil} },
		SyncConflicted: func(conflict Conflict) struct{Name string; Conflict *Conflict} { return struct{Name string; Conflict *Conflict}{"conflicted", &conflict} },
	}).Name, SyncStateFold(sync, SyncStateCases[*Conflict]{
		SyncIdle: func() *Conflict { return nil }, SyncPending: func() *Conflict { return nil },
		SyncRunning: func() *Conflict { return nil },
		SyncConflicted: func(conflict Conflict) *Conflict { return &conflict },
	})
}

func decodeSync(name string, conflict *Conflict) (SyncState, error) {
	switch name {
	case "", "idle": return SyncIdle(), nil
	case "pending": return SyncPending(), nil
	case "running": return SyncRunning(), nil
	case "conflicted":
		if conflict == nil { return nil, fmt.Errorf("forgeflow: sync conflict missing") }
		return SyncConflicted(*conflict), nil
	default: return nil, fmt.Errorf("forgeflow: invalid sync state %q", name)
	}
}

func nowLabel() string { return time.Now().UTC().Format("15:04") }

func cloneIncidents(incidents []Incident) []Incident {
	out := make([]Incident, len(incidents))
	for i, incident := range incidents {
		out[i] = incident
		out[i].Tasks = append([]Task(nil), incident.Tasks...)
		out[i].Timeline = append([]Activity(nil), incident.Timeline...)
	}
	return out
}

func selectedIncident(model Model) (Incident, bool) {
	for _, incident := range model.Incidents {
		if incident.ID == model.SelectedID { return incident, true }
	}
	return Incident{}, false
}

func containsFold(value, query string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(strings.TrimSpace(query)))
}
