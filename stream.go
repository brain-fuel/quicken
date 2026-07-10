package quicken

import "net/http"

// StreamHTML flushes the shell with skeletons immediately, renders regions
// concurrently, and streams each region's real HTML as it becomes ready.
type StreamHTML struct{}

// Deliver implements Transport. Filled in Task 6.
func (StreamHTML) Deliver(w http.ResponseWriter, r *http.Request, p *Page) error {
	return nil
}
