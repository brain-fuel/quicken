//go:build js && wasm

package dom

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall/js"

	"goforge.dev/cadence/program"
	browser "goforge.dev/quicken/web/browser"
)

var ErrUnavailable = errors.New("quicken/web/browser/dom: browser globals are unavailable")

type domState[Model any, Msg any] struct {
	mu          sync.Mutex
	manifest    browser.Manifest
	mount       js.Value
	root        js.Value
	core        *program.Runtime[Model, Msg]
	view        program.View[Model, Msg, browser.Node[Msg]]
	current     browser.Node[Msg]
	hasCurrent  bool
	handlers    map[string]browser.Event[Msg]
	listeners   map[string]js.Func
	composing   map[string]bool
	dispatch    program.Dispatch[Msg]
	failure     error
	unmounted   bool
}

func ReadManifests() ([]browser.Manifest, error) {
	raw := js.Global().Get("__cadenceManifests")
	if raw.Type() == js.TypeUndefined || raw.Type() == js.TypeNull {
		return nil, ErrUnavailable
	}
	encoded := js.Global().Get("JSON").Call("stringify", raw).String()
	var manifests []browser.Manifest
	if err := json.Unmarshal([]byte(encoded), &manifests); err != nil {
		return nil, err
	}
	for _, manifest := range manifests {
		if err := manifest.Validate(); err != nil {
			return nil, err
		}
	}
	return manifests, nil
}

func Mount[Model any, Msg any](options Options[Model, Msg]) (*Runtime[Model, Msg], error) {
	if err := options.Manifest.Validate(); err != nil {
		return nil, err
	}
	document := js.Global().Get("document")
	if document.Type() == js.TypeUndefined || document.Type() == js.TypeNull {
		return nil, ErrUnavailable
	}
	mount := document.Call("getElementById", options.Manifest.MountID)
	if mount.Type() == js.TypeUndefined || mount.Type() == js.TypeNull {
		return nil, fmt.Errorf("quicken/web/browser/dom: mount %q not found", options.Manifest.MountID)
	}
	core := program.NewRuntime(options.Logic, options.Executor)
	state := &domState[Model, Msg]{
		manifest: options.Manifest, mount: mount, core: core, view: options.View,
		handlers: map[string]browser.Event[Msg]{},
		listeners: map[string]js.Func{},
		composing: map[string]bool{},
	}
	state.dispatch = core.Dispatch
	core.SetObserver(func(model Model) {
		state.commit(options.View(model))
	})
	core.Start()
	if err := state.err(); err != nil {
		state.unmount()
		return nil, err
	}
	runtime := &Runtime[Model, Msg]{
		core: core,
		unmount: state.unmount,
		err: state.err,
		setSink: state.setMessageSink,
	}
	return runtime, nil
}

func (s *domState[Model, Msg]) commit(next browser.Node[Msg]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil || s.unmounted {
		return
	}
	if !s.hasCurrent {
		if s.manifest.Plan.Initial == "none" {
			s.root = s.createNode(next, nil)
			s.mount.Call("appendChild", s.root)
		} else {
			s.root = s.mount
			if err := s.hydrateNode(s.root, next, nil); err != nil {
				s.failure = err
				return
			}
		}
		s.current = next
		s.hasCurrent = true
		s.rebuildHandlers(next)
		s.installListeners()
		return
	}

	focus := captureFocus(s.root)
	patches := browser.Diff(s.current, next)
	for _, patch := range patches {
		if err := s.apply(patch); err != nil {
			s.failure = err
			return
		}
	}
	s.current = next
	s.rebuildHandlers(next)
	restoreFocus(s.root, focus)
}

func (s *domState[Model, Msg]) hydrateNode(value js.Value, node browser.Node[Msg], path []int) error {
	switch browser.NodeKindName(node.Kind) {
	case "text":
		if node.Text == "" && value.Get("nodeType").Int() == 8 &&
			value.Get("nodeValue").String() == browser.EmptyTextMarker {
			parent := value.Get("parentNode")
			empty := js.Global().Get("document").Call("createTextNode", "")
			parent.Call("replaceChild", empty, value)
			return nil
		}
		if value.Get("nodeType").Int() != 3 ||
			value.Get("nodeValue").String() != node.Text {
			return fmt.Errorf("quicken/web/browser/dom: hydration text mismatch at %s", pathName(path))
		}
		return nil
	case "fragment":
		if value.Get("nodeType").Int() != 1 ||
			!strings.EqualFold(value.Get("tagName").String(), "cadence-fragment") {
			return fmt.Errorf("quicken/web/browser/dom: hydration fragment mismatch at %s", pathName(path))
		}
	case "element":
		if value.Get("nodeType").Int() != 1 ||
			!strings.EqualFold(value.Get("tagName").String(), node.Tag) {
			return fmt.Errorf("quicken/web/browser/dom: hydration element mismatch at %s", pathName(path))
		}
	default:
		return fmt.Errorf("quicken/web/browser/dom: unknown node kind at %s", pathName(path))
	}
	children := value.Get("childNodes")
	if children.Length() != len(node.Children) {
		return fmt.Errorf("quicken/web/browser/dom: hydration child count mismatch at %s", pathName(path))
	}
	for index, child := range node.Children {
		if err := s.hydrateNode(children.Index(index), child, childPath(path, index)); err != nil {
			return err
		}
	}
	return nil
}

func (s *domState[Model, Msg]) createNode(node browser.Node[Msg], path []int) js.Value {
	document := js.Global().Get("document")
	switch browser.NodeKindName(node.Kind) {
	case "text":
		return document.Call("createTextNode", node.Text)
	case "fragment":
		value := document.Call("createElement", "cadence-fragment")
		value.Get("style").Set("display", "contents")
		s.configureElement(value, node, path)
		return value
	case "element":
		value := document.Call("createElement", node.Tag)
		s.configureElement(value, node, path)
		return value
	default:
		return document.Call("createTextNode", "")
	}
}

func (s *domState[Model, Msg]) configureElement(value js.Value, node browser.Node[Msg], path []int) {
	value.Call("setAttribute", "data-cadence-node", pathName(path))
	if node.Key != "" {
		value.Call("setAttribute", "data-cadence-key", node.Key)
	}
	for _, attribute := range node.Attributes {
		switch browser.AttributeKindName(attribute.Kind) {
		case "attribute":
			value.Call("setAttribute", attribute.Name, attribute.Value)
		case "property":
			value.Set(attribute.Name, attribute.Value)
		case "boolean":
			if attribute.Value == "true" {
				value.Call("setAttribute", attribute.Name, "")
				value.Set(attribute.Name, true)
			}
		}
	}
	for _, event := range node.Events {
		value.Call("setAttribute", "data-cadence-on-"+browser.EventKindName(event.Kind), event.ID)
	}
	for index, child := range node.Children {
		value.Call("appendChild", s.createNode(child, childPath(path, index)))
	}
}

func (s *domState[Model, Msg]) apply(patch browser.Patch[Msg]) error {
	target := resolvePath(s.root, patch.Path)
	if target.Type() == js.TypeUndefined || target.Type() == js.TypeNull {
		return fmt.Errorf("quicken/web/browser/dom: patch path %s not found", pathName(patch.Path))
	}
	switch browser.PatchKindName(patch.Kind) {
	case "replace":
		replacement := s.createNode(patch.Node, patch.Path)
		parent := target.Get("parentNode")
		if parent.Type() == js.TypeNull {
			s.mount.Call("replaceChild", replacement, target)
			s.root = replacement
		} else {
			parent.Call("replaceChild", replacement, target)
			if len(patch.Path) == 0 {
				s.root = replacement
			}
		}
	case "set-text":
		target.Set("nodeValue", patch.Value)
	case "set-attribute":
		target.Call("setAttribute", patch.Name, patch.Value)
	case "remove-attribute":
		target.Call("removeAttribute", patch.Name)
	case "set-property":
		if patch.Name != "value" || !s.isComposing(target) {
			target.Set(patch.Name, patch.Value)
		}
	case "remove-property":
		target.Set(patch.Name, "")
	case "set-events":
		s.syncEventAttributes(target, patch.Events)
	case "insert-child":
		child := s.createNode(patch.Node, childPath(patch.Path, patch.Index))
		children := target.Get("childNodes")
		if patch.Index >= children.Length() {
			target.Call("appendChild", child)
		} else {
			target.Call("insertBefore", child, children.Index(patch.Index))
		}
	case "remove-child":
		children := target.Get("childNodes")
		if patch.Index < 0 || patch.Index >= children.Length() {
			return fmt.Errorf("quicken/web/browser/dom: remove index %d out of range", patch.Index)
		}
		target.Call("removeChild", children.Index(patch.Index))
	case "move-child":
		children := target.Get("childNodes")
		if patch.From < 0 || patch.From >= children.Length() {
			return fmt.Errorf("quicken/web/browser/dom: move index %d out of range", patch.From)
		}
		child := children.Index(patch.From)
		target.Call("removeChild", child)
		children = target.Get("childNodes")
		if patch.Index >= children.Length() {
			target.Call("appendChild", child)
		} else {
			target.Call("insertBefore", child, children.Index(patch.Index))
		}
	default:
		return fmt.Errorf("quicken/web/browser/dom: unknown patch kind")
	}
	return nil
}

func (s *domState[Model, Msg]) syncEventAttributes(target js.Value, events []browser.Event[Msg]) {
	names := []string{"click", "input", "change", "submit", "focus", "blur", "keydown", "keyup"}
	for _, name := range names {
		target.Call("removeAttribute", "data-cadence-on-"+name)
	}
	for _, event := range events {
		target.Call("setAttribute", "data-cadence-on-"+browser.EventKindName(event.Kind), event.ID)
	}
}

func (s *domState[Model, Msg]) rebuildHandlers(root browser.Node[Msg]) {
	next := make(map[string]browser.Event[Msg])
	var walk func(browser.Node[Msg])
	walk = func(node browser.Node[Msg]) {
		for _, event := range node.Events {
			next[event.ID] = event
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(root)
	s.handlers = next
}

func (s *domState[Model, Msg]) installListeners() {
	for _, name := range []string{"click", "input", "change", "submit", "focus", "blur", "keydown", "keyup"} {
		eventName := name
		callback := js.FuncOf(func(this js.Value, args []js.Value) any {
			s.handleEvent(eventName, args[0])
			return nil
		})
		s.root.Call("addEventListener", eventName, callback, true)
		s.listeners[eventName] = callback
	}
	for _, name := range []string{"compositionstart", "compositionend"} {
		eventName := name
		callback := js.FuncOf(func(this js.Value, args []js.Value) any {
			target := args[0].Get("target")
			id := target.Call("getAttribute", "data-cadence-node").String()
			s.mu.Lock()
			s.composing[id] = eventName == "compositionstart"
			s.mu.Unlock()
			return nil
		})
		s.root.Call("addEventListener", eventName, callback, true)
		s.listeners[eventName] = callback
	}
}

func (s *domState[Model, Msg]) handleEvent(name string, event js.Value) {
	s.mu.Lock()
	if s.unmounted || s.failure != nil {
		s.mu.Unlock()
		return
	}
	target := event.Get("target")
	var handler browser.Event[Msg]
	found := false
	for target.Type() != js.TypeNull && target.Type() != js.TypeUndefined {
		if target.Get("nodeType").Int() == 1 {
			id := target.Call("getAttribute", "data-cadence-on-"+name).String()
			if id != "" {
				handler, found = s.handlers[id]
				break
			}
		}
		if target.Equal(s.root) {
			break
		}
		target = target.Get("parentNode")
	}
	s.mu.Unlock()
	if !found {
		return
	}
	if name == "submit" {
		event.Call("preventDefault")
	}
	data := eventData(event)
	var msg Msg
	if handler.Decode != nil {
		msg = handler.Decode(data)
	}
	s.mu.Lock()
	dispatch := s.dispatch
	s.mu.Unlock()
	if dispatch != nil {
		dispatch(msg)
	}
}

func (s *domState[Model, Msg]) setMessageSink(sink program.Dispatch[Msg]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sink == nil {
		s.dispatch = s.core.Dispatch
		return
	}
	s.dispatch = sink
}

func eventData(event js.Value) browser.EventData {
	target := event.Get("target")
	data := browser.EventData{
		Key: event.Get("key").String(),
		Code: event.Get("code").String(),
		AltKey: event.Get("altKey").Bool(),
		CtrlKey: event.Get("ctrlKey").Bool(),
		MetaKey: event.Get("metaKey").Bool(),
		ShiftKey: event.Get("shiftKey").Bool(),
		Repeat: event.Get("repeat").Bool(),
	}
	if target.Type() != js.TypeNull && target.Type() != js.TypeUndefined {
		data.Value = target.Get("value").String()
		data.Checked = target.Get("checked").Bool()
	}
	return data
}

func (s *domState[Model, Msg]) isComposing(target js.Value) bool {
	id := target.Call("getAttribute", "data-cadence-node").String()
	return s.composing[id]
}

func (s *domState[Model, Msg]) err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failure
}

func (s *domState[Model, Msg]) unmount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unmounted {
		return
	}
	for name, callback := range s.listeners {
		s.root.Call("removeEventListener", name, callback, true)
		callback.Release()
	}
	s.listeners = nil
	s.handlers = nil
	s.unmounted = true
}

type focusState struct {
	Key       string
	Node      string
	Start     int
	End       int
	Direction string
	Valid     bool
}

func captureFocus(root js.Value) focusState {
	active := js.Global().Get("document").Get("activeElement")
	if active.Type() == js.TypeNull || active.Type() == js.TypeUndefined ||
		!root.Call("contains", active).Bool() {
		return focusState{}
	}
	state := focusState{
		Key: active.Call("getAttribute", "data-cadence-key").String(),
		Node: active.Call("getAttribute", "data-cadence-node").String(),
		Valid: true,
	}
	if active.Get("selectionStart").Type() == js.TypeNumber {
		state.Start = active.Get("selectionStart").Int()
		state.End = active.Get("selectionEnd").Int()
		state.Direction = active.Get("selectionDirection").String()
	}
	return state
}

func restoreFocus(root js.Value, state focusState) {
	if !state.Valid {
		return
	}
	active := js.Global().Get("document").Get("activeElement")
	if active.Type() != js.TypeNull && active.Type() != js.TypeUndefined &&
		root.Call("contains", active).Bool() {
		return
	}
	var target js.Value
	if state.Key != "" {
		target = root.Call("querySelector", `[data-cadence-key="`+cssEscape(state.Key)+`"]`)
	}
	if (target.Type() == js.TypeNull || target.Type() == js.TypeUndefined) && state.Node != "" {
		target = root.Call("querySelector", `[data-cadence-node="`+cssEscape(state.Node)+`"]`)
	}
	if target.Type() == js.TypeNull || target.Type() == js.TypeUndefined {
		return
	}
	target.Call("focus")
	if target.Get("setSelectionRange").Type() == js.TypeFunction {
		target.Call("setSelectionRange", state.Start, state.End, state.Direction)
	}
}

func resolvePath(root js.Value, path []int) js.Value {
	current := root
	for _, index := range path {
		children := current.Get("childNodes")
		if index < 0 || index >= children.Length() {
			return js.Undefined()
		}
		current = children.Index(index)
	}
	return current
}

func cssEscape(value string) string {
	css := js.Global().Get("CSS")
	if css.Type() != js.TypeUndefined && css.Get("escape").Type() == js.TypeFunction {
		return css.Call("escape", value).String()
	}
	return strings.ReplaceAll(value, `"`, `\"`)
}

func pathName(path []int) string {
	if len(path) == 0 {
		return "root"
	}
	parts := make([]string, len(path))
	for index, value := range path {
		parts[index] = fmt.Sprint(value)
	}
	return strings.Join(parts, ".")
}

func childPath(path []int, index int) []int {
	out := make([]int, len(path)+1)
	copy(out, path)
	out[len(path)] = index
	return out
}
