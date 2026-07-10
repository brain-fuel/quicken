package quicken

import (
	"encoding/json"
	"strconv"
	"strings"
)

// clientMsg is a message decoded from the client over the live connection.
// Type is "event" (a user interaction dispatched to a live region) or
// "resume" (reattach to a prior session using Token).
type clientMsg struct {
	Type    string  `json:"type"`
	Region  string  `json:"region,omitempty"`
	Event   string  `json:"event,omitempty"`
	Payload Payload `json:"payload,omitempty"`
	Token   string  `json:"token,omitempty"`
}

// serverMsg is a message encoded to the client over the live connection.
// Type is "first" (initial statics+dynamics for a region), "full" (a full
// re-render, same shape as first), "patch" (only changed dynamic slots), or
// "error".
type serverMsg struct {
	Type     string   `json:"type"`
	Region   string   `json:"region"`
	Statics  []string `json:"statics,omitempty"`
	Dynamics []string `json:"dynamics,omitempty"`
	// Changed maps dynamic slot index to its new value. json.Marshal encodes
	// map[int]string keys as strings, so this serializes as e.g.
	// {"0":"8"}; the client parses the key with parseInt.
	Changed map[int]string `json:"changed,omitempty"`
	Message string         `json:"message,omitempty"`
}

// decodeClient parses a clientMsg from raw bytes received on the wire.
func decodeClient(b []byte) (clientMsg, error) {
	var m clientMsg
	err := json.Unmarshal(b, &m)
	return m, err
}

// encodeServer serializes a serverMsg for sending on the wire.
func encodeServer(m serverMsg) ([]byte, error) { return json.Marshal(m) }

// treeStatics exposes a Tree's statics to the wire layer.
func treeStatics(t Tree) []string { return t.statics }

// treeDynamics exposes a Tree's dynamics to the wire layer.
func treeDynamics(t Tree) []string { return t.dynamics }

// firstMsg builds the initial message for a region: its full statics and
// dynamics, sent once when a client attaches.
func firstMsg(region string, t Tree) serverMsg {
	return serverMsg{Type: "first", Region: region, Statics: t.statics, Dynamics: t.dynamics}
}

// fullMsg builds a full re-render message, same shape as firstMsg, sent when
// a region's static shape has changed and a patch is not possible.
func fullMsg(region string, t Tree) serverMsg {
	return serverMsg{Type: "full", Region: region, Statics: t.statics, Dynamics: t.dynamics}
}

// patchMsg builds a message carrying only the dynamic slots that changed.
func patchMsg(region string, changed map[int]string) serverMsg {
	return serverMsg{Type: "patch", Region: region, Changed: changed}
}

// errorMsg builds an error message for a region.
func errorMsg(region, message string) serverMsg {
	return serverMsg{Type: "error", Region: region, Message: message}
}

// renderLiveHTML stitches a Tree, wrapping each dynamic slot in a marker
// element the client can address by index when applying a patch.
func renderLiveHTML(t Tree) string {
	if len(t.dynamics) == 0 {
		if len(t.statics) == 0 {
			return ""
		}
		return t.statics[0]
	}
	var b strings.Builder
	for i, d := range t.dynamics {
		b.WriteString(t.statics[i])
		b.WriteString(`<q-d data-qi="`)
		b.WriteString(strconv.Itoa(i))
		b.WriteString(`">`)
		b.WriteString(d)
		b.WriteString(`</q-d>`)
	}
	b.WriteString(t.statics[len(t.statics)-1])
	return b.String()
}
