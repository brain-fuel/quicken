package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
)

// TextInputBridge composes Bubbles' terminal-local editing mechanics around a
// shared domain model. A changed value is lifted into the application's Msg.
type TextInputBridge[Msg any] struct {
	Model   textinput.Model
	Changed func(string) Msg
}

func NewTextInput[Msg any](changed func(string) Msg) TextInputBridge[Msg] {
	if changed == nil {
		panic("quicken/tui: text input change mapper must be non-nil")
	}
	return TextInputBridge[Msg]{Model: textinput.New(), Changed: changed}
}

func (b TextInputBridge[Msg]) Update(
	message tea.Msg,
) (TextInputBridge[Msg], tea.Cmd, Msg, bool) {
	before := b.Model.Value()
	next, command := b.Model.Update(message)
	b.Model = next
	if next.Value() != before {
		return b, command, b.Changed(next.Value()), true
	}
	var zero Msg
	return b, command, zero, false
}

func (b TextInputBridge[Msg]) View() string { return b.Model.View() }
func (b TextInputBridge[Msg]) Value() string { return b.Model.Value() }
