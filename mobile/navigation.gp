package mobile

type Navigation[Route any] struct {
	stack []Route
}

func NewNavigation[Route any](root Route) Navigation[Route] {
	return Navigation[Route]{stack: []Route{root}}
}

func (n Navigation[Route]) Current() Route {
	return n.stack[len(n.stack)-1]
}

func (n Navigation[Route]) Depth() int { return len(n.stack) }

func (n Navigation[Route]) Push(route Route) Navigation[Route] {
	next := append([]Route(nil), n.stack...)
	next = append(next, route)
	return Navigation[Route]{stack: next}
}

func (n Navigation[Route]) Replace(route Route) Navigation[Route] {
	next := append([]Route(nil), n.stack...)
	next[len(next)-1] = route
	return Navigation[Route]{stack: next}
}

func (n Navigation[Route]) Pop() Navigation[Route] {
	if len(n.stack) <= 1 {
		return n
	}
	next := append([]Route(nil), n.stack[:len(n.stack)-1]...)
	return Navigation[Route]{stack: next}
}
