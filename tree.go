package quicken

import "strings"

// Tree is a rendered region as an interleaving of static and dynamic parts:
// statics[0] + dynamics[0] + statics[1] + ... + statics[n], where
// len(statics) == len(dynamics)+1. Statics are fixed across renders of the
// same region; dynamics are recomputed and are what a later phase diffs.
type Tree struct {
	statics  []string
	dynamics []string
}

// Text returns a Tree with a single static string and no dynamic slots.
func Text(s string) Tree {
	return Tree{statics: []string{s}}
}

// Slots returns a Tree from interleaved statics and dynamics. It panics when
// len(statics) != len(dynamics)+1, which is always a programming error.
func Slots(statics, dynamics []string) Tree {
	if len(statics) != len(dynamics)+1 {
		panic("quicken: Slots requires len(statics) == len(dynamics)+1")
	}
	return Tree{
		statics:  append([]string(nil), statics...),
		dynamics: append([]string(nil), dynamics...),
	}
}

// HTML stitches the tree into a single string.
func (t Tree) HTML() string {
	if len(t.dynamics) == 0 {
		if len(t.statics) == 0 {
			return ""
		}
		return t.statics[0]
	}
	var b strings.Builder
	for i, d := range t.dynamics {
		b.WriteString(t.statics[i])
		b.WriteString(d)
	}
	b.WriteString(t.statics[len(t.statics)-1])
	return b.String()
}

// Diff compares t against a previous tree for the same region. When both trees
// share the same static shape it returns the dynamic slots whose value changed,
// keyed by slot index. When the static shapes differ it returns fullReplace
// true and a nil map, meaning the whole region must be replaced rather than
// patched.
func (t Tree) Diff(prev Tree) (changed map[int]string, fullReplace bool) {
	if !sameStatics(t.statics, prev.statics) {
		return nil, true
	}
	changed = map[int]string{}
	for i, d := range t.dynamics {
		if d != prev.dynamics[i] {
			changed[i] = d
		}
	}
	return changed, false
}

func sameStatics(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
