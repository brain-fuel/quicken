package browser

import "fmt"

type PatchKind enum {
	PatchReplace
	PatchSetText
	PatchSetAttribute
	PatchRemoveAttribute
	PatchSetProperty
	PatchRemoveProperty
	PatchSetEvents
	PatchInsertChild
	PatchRemoveChild
	PatchMoveChild
}

// Patch is a deterministic DOM operation. Path addresses a node from the
// mounted root; Index and From are child positions for structural operations.
type Patch[Msg any] struct {
	Kind      PatchKind
	Path      []int
	Index     int
	From      int
	Key       string
	Name      string
	Value     string
	Node      Node[Msg]
	Events    []Event[Msg]
}

func PatchKindName(kind PatchKind) string {
	name := ""
	match kind {
	case PatchReplace():
		name = "replace"
	case PatchSetText():
		name = "set-text"
	case PatchSetAttribute():
		name = "set-attribute"
	case PatchRemoveAttribute():
		name = "remove-attribute"
	case PatchSetProperty():
		name = "set-property"
	case PatchRemoveProperty():
		name = "remove-property"
	case PatchSetEvents():
		name = "set-events"
	case PatchInsertChild():
		name = "insert-child"
	case PatchRemoveChild():
		name = "remove-child"
	case PatchMoveChild():
		name = "move-child"
	}
	return name
}

func Diff[Msg any](old, next Node[Msg]) []Patch[Msg] {
	patches := make([]Patch[Msg], 0)
	diffAt(old, next, nil, &patches)
	return patches
}

func diffAt[Msg any](old, next Node[Msg], path []int, patches *[]Patch[Msg]) {
	if !sameIdentity(old, next) {
		*patches = append(*patches, Patch[Msg]{
			Kind: PatchReplace(), Path: clonePath(path), Node: next,
		})
		return
	}
	match next.Kind {
	case NodeText():
		if old.Text != next.Text {
			*patches = append(*patches, Patch[Msg]{
				Kind: PatchSetText(), Path: clonePath(path), Value: next.Text,
			})
		}
	case NodeElement():
		diffAttributes(old.Attributes, next.Attributes, path, patches)
		if !sameEvents(old.Events, next.Events) {
			*patches = append(*patches, Patch[Msg]{
				Kind: PatchSetEvents(), Path: clonePath(path),
				Events: append([]Event[Msg](nil), next.Events...),
			})
		}
		diffChildren(old.Children, next.Children, path, patches)
	case NodeFragment():
		diffChildren(old.Children, next.Children, path, patches)
	}
}

func diffAttributes[Msg any](old, next []Attribute, path []int, patches *[]Patch[Msg]) {
	oldMap := attributeMap(old)
	nextMap := attributeMap(next)
	for key, value := range nextMap {
		previous, ok := oldMap[key]
		if ok && previous.Value == value.Value {
			continue
		}
		var kind PatchKind = PatchSetAttribute()
		if attributeKindName(value.Kind) == "property" {
			kind = PatchSetProperty()
		}
		*patches = append(*patches, Patch[Msg]{
			Kind: kind, Path: clonePath(path), Name: value.Name, Value: value.Value,
		})
	}
	for key, value := range oldMap {
		if _, ok := nextMap[key]; ok {
			continue
		}
		var kind PatchKind = PatchRemoveAttribute()
		if attributeKindName(value.Kind) == "property" {
			kind = PatchRemoveProperty()
		}
		*patches = append(*patches, Patch[Msg]{
			Kind: kind, Path: clonePath(path), Name: value.Name,
		})
	}
}

func diffChildren[Msg any](old, next []Node[Msg], path []int, patches *[]Patch[Msg]) {
	if allKeyed(old) && allKeyed(next) {
		diffKeyedChildren(old, next, path, patches)
		return
	}
	common := len(old)
	if len(next) < common {
		common = len(next)
	}
	for index := 0; index < common; index++ {
		diffAt(old[index], next[index], childPath(path, index), patches)
	}
	for index := len(old) - 1; index >= len(next); index-- {
		*patches = append(*patches, Patch[Msg]{
			Kind: PatchRemoveChild(), Path: clonePath(path), Index: index,
		})
	}
	for index := common; index < len(next); index++ {
		*patches = append(*patches, Patch[Msg]{
			Kind: PatchInsertChild(), Path: clonePath(path),
			Index: index, Node: next[index],
		})
	}
}

func diffKeyedChildren[Msg any](old, next []Node[Msg], path []int, patches *[]Patch[Msg]) {
	working := append([]Node[Msg](nil), old...)
	nextKeys := make(map[string]bool, len(next))
	for index, child := range next {
		nextKeys[child.Key] = true
		current := indexOfKey(working, child.Key)
		if current < 0 {
			*patches = append(*patches, Patch[Msg]{
				Kind: PatchInsertChild(), Path: clonePath(path),
				Index: index, Key: child.Key, Node: child,
			})
			working = insertNode(working, index, child)
			continue
		}
		if current != index {
			*patches = append(*patches, Patch[Msg]{
				Kind: PatchMoveChild(), Path: clonePath(path),
				From: current, Index: index, Key: child.Key,
			})
			working = moveNode(working, current, index)
		}
		diffAt(working[index], child, childPath(path, index), patches)
		working[index] = child
	}
	for index := len(working) - 1; index >= 0; index-- {
		if nextKeys[working[index].Key] {
			continue
		}
		*patches = append(*patches, Patch[Msg]{
			Kind: PatchRemoveChild(), Path: clonePath(path),
			Index: index, Key: working[index].Key,
		})
		working = append(working[:index], working[index+1:]...)
	}
}

func sameIdentity[Msg any](old, next Node[Msg]) bool {
	if nodeKindName(old.Kind) != nodeKindName(next.Kind) || old.Key != next.Key {
		return false
	}
	if nodeKindName(next.Kind) == "element" {
		return old.Tag == next.Tag
	}
	return true
}

func sameEvents[Msg any](old, next []Event[Msg]) bool {
	if len(old) != len(next) {
		return false
	}
	for index := range old {
		if old[index].ID != next[index].ID ||
			EventKindName(old[index].Kind) != EventKindName(next[index].Kind) {
			return false
		}
	}
	return true
}

func attributeMap(attributes []Attribute) map[string]Attribute {
	out := make(map[string]Attribute, len(attributes))
	for _, attribute := range attributes {
		key := fmt.Sprintf("%s:%s", attributeKindName(attribute.Kind), attribute.Name)
		out[key] = attribute
	}
	return out
}

func allKeyed[Msg any](nodes []Node[Msg]) bool {
	if len(nodes) == 0 {
		return false
	}
	seen := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		if node.Key == "" || seen[node.Key] {
			return false
		}
		seen[node.Key] = true
	}
	return true
}

func indexOfKey[Msg any](nodes []Node[Msg], key string) int {
	for index, node := range nodes {
		if node.Key == key {
			return index
		}
	}
	return -1
}

func insertNode[Msg any](nodes []Node[Msg], index int, node Node[Msg]) []Node[Msg] {
	nodes = append(nodes, Node[Msg]{})
	copy(nodes[index+1:], nodes[index:])
	nodes[index] = node
	return nodes
}

func moveNode[Msg any](nodes []Node[Msg], from, to int) []Node[Msg] {
	node := nodes[from]
	nodes = append(nodes[:from], nodes[from+1:]...)
	return insertNode(nodes, to, node)
}

func clonePath(path []int) []int {
	return append([]int(nil), path...)
}

func childPath(path []int, index int) []int {
	out := make([]int, len(path)+1)
	copy(out, path)
	out[len(path)] = index
	return out
}
