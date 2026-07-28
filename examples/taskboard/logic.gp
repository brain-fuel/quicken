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
	Tasks []Task `json:"tasks"`
}

type SaveResponse struct {
	Tasks []Task `json:"tasks"`
}

func Logic(initial Model, save SaveEffect) program.Logic[Model, Msg] {
	if save == nil {
		save = LocalSave
	}
	return program.Logic[Model, Msg]{
		Init: func() program.Step[Model, Msg] {
			return program.Step[Model, Msg]{
				Model: initial,
				Command: program.None[Msg](),
			}
		},
		Update: func(model Model, message Msg) program.Step[Model, Msg] {
			return update(model, message, save)
		},
		Subscriptions: func(Model) program.Subscriptions[Msg] {
			return program.Subs[Msg]()
		},
	}
}

func update(model Model, message Msg, save SaveEffect) program.Step[Model, Msg] {
	command := program.None[Msg]()
	match message {
	case DraftChanged(value):
		model.Draft = value
		model.Error = ""
	case AddRequested():
		title := strings.TrimSpace(model.Draft)
		if title == "" {
			model.Error = "A task title is required."
		} else {
			model.Tasks = append(cloneTasks(model.Tasks), Task{
				ID: model.NextID, Title: title,
			})
			model.NextID++
			model.Draft = ""
			model.Saving = true
			model.Error = ""
			command = save(model)
		}
	case ToggleRequested(id):
		model.Tasks = mapTasks(model.Tasks, func(task Task) Task {
			if task.ID == id {
				task.Done = !task.Done
			}
			return task
		})
		model.Saving = true
		command = save(model)
	case DeleteRequested(id):
		next := make([]Task, 0, len(model.Tasks))
		for _, task := range model.Tasks {
			if task.ID != id {
				next = append(next, task)
			}
		}
		model.Tasks = next
		model.Saving = true
		command = save(model)
	case FilterRequested(filter):
		model.Filter = filter
	case SaveSucceeded(tasks):
		model.Tasks = cloneTasks(tasks)
		model.Saving = false
		model.Error = ""
	case SaveFailed(message):
		model.Saving = false
		model.Error = message
	}
	return program.Step[Model, Msg]{Model: model, Command: command}
}

func LocalSave(model Model) program.Cmd[Msg] {
	return program.After(
		50*time.Millisecond,
		program.Emit[Msg](SaveSucceeded(cloneTasks(model.Tasks))),
	)
}

func RemoteSave(model Model) program.Cmd[Msg] {
	requestID := fmt.Sprintf("save-%d-%d", model.NextID, len(model.Tasks))
	return browser.MustRemote(
		requestID,
		"taskboard.save",
		SaveRequest{Tasks: cloneTasks(model.Tasks)},
		func(response SaveResponse) Msg {
			return SaveSucceeded(cloneTasks(response.Tasks))
		},
		func(failure browser.PublicError) Msg {
			return SaveFailed(failure.Message)
		},
	)
}

func VisibleTasks(model Model) []Task {
	out := make([]Task, 0, len(model.Tasks))
	for _, task := range model.Tasks {
		include := FilterFold(model.Filter, FilterCases[bool]{
			FilterAll: func() bool { return true },
			FilterActive: func() bool { return !task.Done },
			FilterCompleted: func() bool { return task.Done },
		})
		if include {
			out = append(out, task)
		}
	}
	return out
}

func mapTasks(tasks []Task, transform func(Task) Task) []Task {
	out := make([]Task, len(tasks))
	for index, task := range tasks {
		out[index] = transform(task)
	}
	return out
}
