package quicken

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// defaultPollTimeout is how long a poll blocks for the next message when
// LiveChannel.pollTimeout is unset.
const defaultPollTimeout = 25 * time.Second

// livePollPath is the long-poll base path for a page: /_live/poll for an
// unnamed page, or /_live/<name>/poll when the page is named. The poll and
// event endpoints are mounted at this path plus "/poll" and "/event".
func livePollPath(name string) string {
	if name == "" {
		return liveBase + "/poll"
	}
	return liveBase + "/" + name + "/poll"
}

// timeout returns the configured poll timeout, or defaultPollTimeout when
// unset.
func (lc LiveChannel) timeout() time.Duration {
	if lc.pollTimeout > 0 {
		return lc.pollTimeout
	}
	return defaultPollTimeout
}

// pollHandler serves GET .../poll?token=...: on the first poll for a session
// it enqueues the "first" message for every live region, then blocks on the
// session's outbox up to the configured timeout. It responds with the next
// queued message as JSON, 204 on timeout, or 404 for an unknown token.
func (lc LiveChannel) pollHandler(p *Page) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		sess, ok := lc.store().Get(token)
		if !ok {
			http.NotFound(w, r)
			return
		}
		if sess.markFirstSent() {
			for _, lr := range p.liveRegions() {
				var fm serverMsg
				ok := sess.withRegion(lr.ID(), func(rs *regionState) {
					fm = firstMsg(lr.ID(), Tree{statics: lr.Render(rs.state).statics, dynamics: rs.lastDynamics})
				})
				if ok {
					sess.outbox <- fm
				}
			}
		}
		select {
		case m := <-sess.outbox:
			b, err := encodeServer(m)
			if err != nil {
				http.Error(w, "encode error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		case <-time.After(lc.timeout()):
			w.WriteHeader(http.StatusNoContent)
		case <-r.Context().Done():
			w.WriteHeader(http.StatusNoContent)
		}
	})
}

// eventHandler serves POST .../event: it decodes a clientMsg event, applies
// it to the named region under the session lock, and enqueues the resulting
// serverMsg on the session's outbox (dropping the oldest queued message if
// the outbox is full). It responds 404 for an unknown token or region, 400
// for a malformed body, and 204 on success.
func (lc LiveChannel) eventHandler(p *Page) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		var m clientMsg
		if err := json.Unmarshal(body, &m); err != nil || m.Type != "event" {
			http.Error(w, "bad event", http.StatusBadRequest)
			return
		}
		sess, ok := lc.store().Get(m.Token)
		if !ok {
			http.NotFound(w, r)
			return
		}
		lr, ok := p.live[m.Region]
		if !ok {
			http.NotFound(w, r)
			return
		}
		ctx := RenderContext{Ctx: r.Context(), R: r}
		var out serverMsg
		found := sess.withRegion(m.Region, func(rs *regionState) {
			out = lc.applyEvent(lr, ctx, rs, m)
		})
		if !found {
			http.NotFound(w, r)
			return
		}
		select {
		case sess.outbox <- out:
		default:
			// Outbox full: drop the oldest queued message, then enqueue.
			select {
			case <-sess.outbox:
			default:
			}
			select {
			case sess.outbox <- out:
			default:
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
