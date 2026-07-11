package quicken

import "strconv"

// testClock is a minimal LiveRegion shared across live tests (runtime_test.go,
// livechannel_test.go, and any future live test) so no test file duplicates
// its own trivial clock region. State is an int tick counter; Render shows
// "TICK-<n>"; a "tick" event increments it.
type testClock struct{ id string }

// newTestClock returns a testClock LiveRegion with the given id.
func newTestClock(id string) LiveRegion { return testClock{id: id} }

func (c testClock) ID() string { return c.id }

func (c testClock) Skeleton(RenderContext) Tree { return Text(`<span>...</span>`) }

func (c testClock) Mount(RenderContext, Params) (State, error) { return 0, nil }

func (c testClock) HandleEvent(_ RenderContext, name string, _ Payload, s State) (State, error) {
	if name == "tick" {
		return s.(int) + 1, nil
	}
	return s, nil
}

func (c testClock) Render(s State) Tree {
	return Text("TICK-" + strconv.Itoa(s.(int)))
}
