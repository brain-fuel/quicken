package taskboard

import (
	"encoding/json"
	"fmt"

	"goforge.dev/cadence/program"
)

type Task struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type Filter enum {
	FilterAll
	FilterActive
	FilterCompleted
}

type Model struct {
	Tasks  []Task
	Draft  string
	Filter Filter
	Saving bool
	Error  string
	NextID int
}

//goplus:derive gen
type Msg enum {
	DraftChanged(value string)
	AddRequested
	ToggleRequested(id int)
	DeleteRequested(id int)
	FilterRequested(filter Filter)
	SaveSucceeded(tasks []Task)
	SaveFailed(message string)
}

type Bootstrap struct {
	Initial Model
}

type modelWire struct {
	Tasks  []Task `json:"tasks"`
	Draft  string `json:"draft"`
	Filter string `json:"filter"`
	Saving bool   `json:"saving"`
	Error  string `json:"error"`
	NextID int    `json:"next_id"`
}

type messageWire struct {
	Type    string `json:"type"`
	Value   string `json:"value,omitempty"`
	ID      int    `json:"id,omitempty"`
	Filter  string `json:"filter,omitempty"`
	Tasks   []Task `json:"tasks,omitempty"`
	Message string `json:"message,omitempty"`
}

func InitialModel() Model {
	return Model{
		Tasks: []Task{
			{ID: 1, Title: "Define the shared model"},
			{ID: 2, Title: "Hydrate browser and native views"},
		},
		Filter: FilterAll(),
		NextID: 3,
	}
}

func ModelCodec() program.Codec[Model] {
	return program.Codec[Model]{
		Encode: func(model Model) ([]byte, error) {
			return json.Marshal(modelWire{
				Tasks: cloneTasks(model.Tasks),
				Draft: model.Draft,
				Filter: filterName(model.Filter),
				Saving: model.Saving,
				Error: model.Error,
				NextID: model.NextID,
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
			return Model{
				Tasks: cloneTasks(wire.Tasks),
				Draft: wire.Draft,
				Filter: filter,
				Saving: wire.Saving,
				Error: wire.Error,
				NextID: wire.NextID,
			}, nil
		},
	}
}

func MessageCodec() program.Codec[Msg] {
	return program.Codec[Msg]{
		Encode: encodeMessage,
		Decode: decodeMessage,
	}
}

func encodeMessage(message Msg) ([]byte, error) {
	var wire messageWire
	match message {
	case DraftChanged(value):
		wire = messageWire{Type: "draft-changed", Value: value}
	case AddRequested():
		wire = messageWire{Type: "add-requested"}
	case ToggleRequested(id):
		wire = messageWire{Type: "toggle-requested", ID: id}
	case DeleteRequested(id):
		wire = messageWire{Type: "delete-requested", ID: id}
	case FilterRequested(filter):
		wire = messageWire{Type: "filter-requested", Filter: filterName(filter)}
	case SaveSucceeded(tasks):
		wire = messageWire{Type: "save-succeeded", Tasks: cloneTasks(tasks)}
	case SaveFailed(message):
		wire = messageWire{Type: "save-failed", Message: message}
	}
	return json.Marshal(wire)
}

func decodeMessage(data []byte) (Msg, error) {
	var wire messageWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, err
	}
	switch wire.Type {
	case "draft-changed":
		return DraftChanged(wire.Value), nil
	case "add-requested":
		return AddRequested(), nil
	case "toggle-requested":
		return ToggleRequested(wire.ID), nil
	case "delete-requested":
		return DeleteRequested(wire.ID), nil
	case "filter-requested":
		filter, err := parseFilter(wire.Filter)
		if err != nil {
			return nil, err
		}
		return FilterRequested(filter), nil
	case "save-succeeded":
		return SaveSucceeded(cloneTasks(wire.Tasks)), nil
	case "save-failed":
		return SaveFailed(wire.Message), nil
	default:
		return nil, fmt.Errorf("taskboard: unknown message %q", wire.Type)
	}
}

func filterName(filter Filter) string {
	return FilterFold(filter, FilterCases[string]{
		FilterAll: func() string { return "all" },
		FilterActive: func() string { return "active" },
		FilterCompleted: func() string { return "completed" },
	})
}

func parseFilter(value string) (Filter, error) {
	switch value {
	case "", "all":
		return FilterAll(), nil
	case "active":
		return FilterActive(), nil
	case "completed":
		return FilterCompleted(), nil
	default:
		return nil, fmt.Errorf("taskboard: invalid filter %q", value)
	}
}

func cloneTasks(tasks []Task) []Task {
	return append([]Task(nil), tasks...)
}
