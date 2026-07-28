package browser

import "goforge.dev/cadence/style"

type NodeKind enum {
	NodeElement
	NodeText
	NodeFragment
}

type AttributeKind enum {
	AttributeString
	AttributeProperty
	AttributeBoolean
}

type Attribute struct {
	Kind  AttributeKind
	Name  string
	Value string
}

type EventKind enum {
	EventClick
	EventInput
	EventChange
	EventSubmit
	EventFocus
	EventBlur
	EventKeyDown
	EventKeyUp
}

// EventData is the browser-neutral input exposed to application decoders.
type EventData struct {
	Value     string
	Key       string
	Code      string
	Checked   bool
	AltKey    bool
	CtrlKey   bool
	MetaKey   bool
	ShiftKey  bool
	Repeat    bool
}

// Event has a stable ID for delegated listener reconciliation. Decode turns
// browser-neutral data into the application's closed message type.
type Event[Msg any] struct {
	ID      string
	Kind    EventKind
	Decode  func(EventData) Msg
}

// Node is the browser-native declarative tree. It contains no DOM handles.
type Node[Msg any] struct {
	Kind       NodeKind
	Key        string
	Tag        string
	Text       string
	Attributes []Attribute
	Events     []Event[Msg]
	Children   []Node[Msg]
	Style      style.Style
}

func Element[Msg any](tag string, children ...Node[Msg]) Node[Msg] {
	return Node[Msg]{
		Kind: NodeElement(), Tag: tag,
		Children: cloneNodes(children), Style: style.Default(),
	}
}

func Text[Msg any](value string) Node[Msg] {
	return Node[Msg]{Kind: NodeText(), Text: value, Style: style.Default()}
}

func Fragment[Msg any](children ...Node[Msg]) Node[Msg] {
	return Node[Msg]{
		Kind: NodeFragment(), Children: cloneNodes(children),
		Style: style.Default(),
	}
}

func (n Node[Msg]) WithKey(key string) Node[Msg] {
	n.Key = key
	return n
}

func (n Node[Msg]) WithAttribute(name, value string) Node[Msg] {
	n.Attributes = appendAttribute(n.Attributes, Attribute{
		Kind: AttributeString(), Name: name, Value: value,
	})
	return n
}

func (n Node[Msg]) WithProperty(name, value string) Node[Msg] {
	n.Attributes = appendAttribute(n.Attributes, Attribute{
		Kind: AttributeProperty(), Name: name, Value: value,
	})
	return n
}

func (n Node[Msg]) WithBoolean(name string, value bool) Node[Msg] {
	if value {
		n.Attributes = appendAttribute(n.Attributes, Attribute{
			Kind: AttributeBoolean(), Name: name, Value: "true",
		})
		return n
	}
	n.Attributes = removeAttribute(n.Attributes, AttributeBoolean(), name)
	return n
}

func (n Node[Msg]) On(id string, kind EventKind, decode func(EventData) Msg) Node[Msg] {
	events := append([]Event[Msg](nil), n.Events...)
	events = append(events, Event[Msg]{ID: id, Kind: kind, Decode: decode})
	n.Events = events
	return n
}

func (n Node[Msg]) WithStyle(value style.Style) Node[Msg] {
	n.Style = value
	return n
}

func cloneNodes[Msg any](nodes []Node[Msg]) []Node[Msg] {
	return append([]Node[Msg](nil), nodes...)
}

func appendAttribute(attributes []Attribute, next Attribute) []Attribute {
	out := make([]Attribute, 0, len(attributes)+1)
	for _, current := range attributes {
		if current.Name == next.Name && attributeKindName(current.Kind) == attributeKindName(next.Kind) {
			continue
		}
		out = append(out, current)
	}
	return append(out, next)
}

func removeAttribute(attributes []Attribute, kind AttributeKind, name string) []Attribute {
	out := make([]Attribute, 0, len(attributes))
	for _, current := range attributes {
		if current.Name == name && attributeKindName(current.Kind) == attributeKindName(kind) {
			continue
		}
		out = append(out, current)
	}
	return out
}

func nodeKindName(kind NodeKind) string {
	name := ""
	match kind {
	case NodeElement():
		name = "element"
	case NodeText():
		name = "text"
	case NodeFragment():
		name = "fragment"
	}
	return name
}

func NodeKindName(kind NodeKind) string { return nodeKindName(kind) }

func attributeKindName(kind AttributeKind) string {
	name := ""
	match kind {
	case AttributeString():
		name = "attribute"
	case AttributeProperty():
		name = "property"
	case AttributeBoolean():
		name = "boolean"
	}
	return name
}

func AttributeKindName(kind AttributeKind) string { return attributeKindName(kind) }

func EventKindName(kind EventKind) string {
	name := ""
	match kind {
	case EventClick():
		name = "click"
	case EventInput():
		name = "input"
	case EventChange():
		name = "change"
	case EventSubmit():
		name = "submit"
	case EventFocus():
		name = "focus"
	case EventBlur():
		name = "blur"
	case EventKeyDown():
		name = "keydown"
	case EventKeyUp():
		name = "keyup"
	}
	return name
}
