package quicken

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// ClientMessage is the decoded client protocol. Resume and Event carry only
// the fields valid for that operation; an impossible hybrid cannot enter the
// runtime.
//
//goplus:derive off
type ClientMessage enum {
	ResumeMessage(token string)
	EventMessage(region string, event string, payload Payload, token string)
}

// ServerMessage is the server protocol. Each alternative owns exactly its
// valid payload, replacing the former struct with a string discriminator and
// seven conditionally meaningful fields.
//
//goplus:derive off
type ServerMessage enum {
	FirstMessage(region string, tree Tree)
	FullMessage(region string, tree Tree)
	PatchMessage(region string, changed map[int]string)
	ErrorMessage(region string, message string)
}

// clientMsg and serverMsg are deliberately confined JSON boundary DTOs.
// They are never queued or interpreted as domain values.
type clientMsg struct {
	Type    string  `json:"type"`
	Region  string  `json:"region,omitempty"`
	Event   string  `json:"event,omitempty"`
	Payload Payload `json:"payload,omitempty"`
	Token   string  `json:"token,omitempty"`
}

type serverMsg struct {
	Type     string   `json:"type"`
	Region   string   `json:"region"`
	Statics  []string `json:"statics,omitempty"`
	Dynamics []string `json:"dynamics,omitempty"`
	Changed  map[int]string `json:"changed,omitempty"`
	Message  string   `json:"message,omitempty"`
}

func decodeClient(b []byte) (ClientMessage, error) {
	var m clientMsg
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	switch m.Type {
	case "resume":
		if m.Token == "" { return nil, errors.New("quicken: resume needs token") }
		return ResumeMessage(m.Token), nil
	case "event":
		if m.Region == "" || m.Event == "" { return nil, errors.New("quicken: event needs region and event") }
		return EventMessage(m.Region, m.Event, m.Payload, m.Token), nil
	default:
		return nil, errors.New("quicken: unknown client message type")
	}
}

func encodeServer(m ServerMessage) ([]byte, error) {
	var wire serverMsg
	match m {
	case FirstMessage(region, tree):
		wire = serverMsg{Type: "first", Region: region, Statics: tree.Statics(), Dynamics: tree.Dynamics()}
	case FullMessage(region, tree):
		wire = serverMsg{Type: "full", Region: region, Statics: tree.Statics(), Dynamics: tree.Dynamics()}
	case PatchMessage(region, changed):
		wire = serverMsg{Type: "patch", Region: region, Changed: changed}
	case ErrorMessage(region, message):
		wire = serverMsg{Type: "error", Region: region, Message: message}
	}
	return json.Marshal(wire)
}

func firstMsg(region string, t Tree) ServerMessage { return FirstMessage(region, t) }
func fullMsg(region string, t Tree) ServerMessage { return FullMessage(region, t) }
func patchMsg(region string, changed map[int]string) ServerMessage { return PatchMessage(region, changed) }
func errorMsg(region, message string) ServerMessage { return ErrorMessage(region, message) }

// renderLiveHTML stitches a Tree, wrapping each dynamic slot in a marker
// element the client can address by index when applying a patch.
func renderLiveHTML(t Tree) string {
	statics := t.Statics()
	dynamics := t.Dynamics()
	if len(dynamics) == 0 {
		if len(statics) == 0 { return "" }
		return statics[0]
	}
	var b strings.Builder
	for i, d := range dynamics {
		b.WriteString(statics[i])
		b.WriteString(`<q-d data-qi="`)
		b.WriteString(strconv.Itoa(i))
		b.WriteString(`">`)
		b.WriteString(d)
		b.WriteString(`</q-d>`)
	}
	b.WriteString(statics[len(statics)-1])
	return b.String()
}
