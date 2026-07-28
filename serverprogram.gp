package quicken

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"goforge.dev/cadence/program"
	browser "goforge.dev/quicken/web/browser"
)

type ProgramSession[Model any, Msg any] struct {
	mu       sync.Mutex
	runtime  *program.Runtime[Model, Msg]
	revision uint64
	model    json.RawMessage
	outbox   chan browser.LiveServerEnvelope
	identity liveIdentity
	codec    program.Codec[Model]
}

type liveIdentity struct {
	AppID      string
	ProgramID  string
	InstanceID string
}

type ProgramSessionStore[Model any, Msg any] interface {
	Get(token string) (*ProgramSession[Model, Msg], bool)
	Put(token string, session *ProgramSession[Model, Msg])
	Delete(token string)
}

type memoryProgramStore[Model any, Msg any] struct {
	mu       sync.RWMutex
	sessions map[string]*ProgramSession[Model, Msg]
}

func NewProgramSessionStore[Model any, Msg any]() ProgramSessionStore[Model, Msg] {
	return &memoryProgramStore[Model, Msg]{
		sessions: map[string]*ProgramSession[Model, Msg]{},
	}
}

func (s *memoryProgramStore[Model, Msg]) Get(token string) (*ProgramSession[Model, Msg], bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[token]
	return session, ok
}

func (s *memoryProgramStore[Model, Msg]) Put(token string, session *ProgramSession[Model, Msg]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = session
}

func (s *memoryProgramStore[Model, Msg]) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

// ServerProgram adapts shared TEA logic to authoritative server ownership.
type ServerProgram[Bootstrap any, Model any, Msg any] struct {
	Definition   Program[Bootstrap, Model, Msg]
	ModelCodec   program.Codec[Model]
	MessageCodec program.Codec[Msg]
	Executor     program.Executor[Msg]
	Store        ProgramSessionStore[Model, Msg]
	Security     BrowserSecurity
	PollTimeout  time.Duration
}

type ServerProgramRegion[Bootstrap any, Model any, Msg any] struct {
	MountID string
	Server  *ServerProgram[Bootstrap, Model, Msg]
}

type ServerFullPageProgram[Bootstrap any, Model any, Msg any] struct {
	MountID string
	Server  *ServerProgram[Bootstrap, Model, Msg]
}

func NewServerProgram[Bootstrap any, Model any, Msg any](
	definition Program[Bootstrap, Model, Msg],
	modelCodec program.Codec[Model],
	messageCodec program.Codec[Msg],
	executor program.Executor[Msg],
) *ServerProgram[Bootstrap, Model, Msg] {
	return &ServerProgram[Bootstrap, Model, Msg]{
		Definition: definition,
		ModelCodec: modelCodec,
		MessageCodec: messageCodec,
		Executor: executor,
		Store: NewProgramSessionStore[Model, Msg](),
		Security: DefaultBrowserSecurity(),
		PollTimeout: 25 * time.Second,
	}
}

func (s *ServerProgram[Bootstrap, Model, Msg]) Region(mountID string) ServerProgramRegion[Bootstrap, Model, Msg] {
	return ServerProgramRegion[Bootstrap, Model, Msg]{MountID: mountID, Server: s}
}

func (s *ServerProgram[Bootstrap, Model, Msg]) FullPage(mountID string) ServerFullPageProgram[Bootstrap, Model, Msg] {
	return ServerFullPageProgram[Bootstrap, Model, Msg]{MountID: mountID, Server: s}
}

func (r ServerProgramRegion[Bootstrap, Model, Msg]) Render(req *http.Request) (ProgramRender, error) {
	definition, err := r.Server.prepare(req)
	if err != nil {
		return ProgramRender{}, err
	}
	return ProgramRegion[Bootstrap, Model, Msg]{
		MountID: r.MountID, Program: definition,
	}.Render(req)
}

func (r ServerProgramRegion[Bootstrap, Model, Msg]) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	rendered, err := r.Render(req)
	if err != nil {
		http.Error(w, "program render failed", http.StatusInternalServerError)
		return
	}
	writeProgramHeaders(w)
	_, _ = w.Write([]byte(
		rendered.HTML + rendered.NoScriptHTML + rendered.ManifestTag + rendered.AssetTags,
	))
}

func (p ServerFullPageProgram[Bootstrap, Model, Msg]) Render(req *http.Request) (ProgramRender, error) {
	definition, err := p.Server.prepare(req)
	if err != nil {
		return ProgramRender{}, err
	}
	return FullPageProgram[Bootstrap, Model, Msg]{
		MountID: p.MountID, Program: definition,
	}.Render(req)
}

func (p ServerFullPageProgram[Bootstrap, Model, Msg]) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	rendered, err := p.Render(req)
	if err != nil {
		http.Error(w, "program render failed", http.StatusInternalServerError)
		return
	}
	writeProgramHeaders(w)
	_, _ = w.Write([]byte(rendered.Document))
}

func (s *ServerProgram[Bootstrap, Model, Msg]) MountRoutes(mux *http.ServeMux) error {
	if err := s.validate(); err != nil {
		return err
	}
	mux.Handle(s.Definition.SocketEndpoint, s.socketHandler())
	mux.Handle(s.Definition.PollEndpoint, s.pollHandler())
	mux.Handle(s.Definition.EventEndpoint, s.eventHandler())
	return nil
}

func (s *ServerProgram[Bootstrap, Model, Msg]) prepare(req *http.Request) (Program[Bootstrap, Model, Msg], error) {
	if err := s.validate(); err != nil {
		return Program[Bootstrap, Model, Msg]{}, err
	}
	bootstrap, err := s.Definition.Bootstrap(req)
	if err != nil {
		return Program[Bootstrap, Model, Msg]{}, err
	}
	token, err := newToken()
	if err != nil {
		return Program[Bootstrap, Model, Msg]{}, err
	}
	instanceID, err := newToken()
	if err != nil {
		return Program[Bootstrap, Model, Msg]{}, err
	}
	identity := liveIdentity{
		AppID: s.Definition.AppID,
		ProgramID: s.Definition.ID,
		InstanceID: instanceID,
	}
	session := &ProgramSession[Model, Msg]{
		outbox: make(chan browser.LiveServerEnvelope, 64),
		identity: identity,
		codec: s.ModelCodec,
	}
	session.runtime = program.NewRuntime(s.Definition.Logic(bootstrap), s.Executor)
	session.runtime.SetObserver(func(model Model) { session.publish("", model) })
	session.runtime.Start()
	s.Store.Put(token, session)

	definition := s.Definition
	definition.InstanceID = func(*http.Request) (string, error) { return instanceID, nil }
	definition.ResumeToken = func(*http.Request) string { return token }
	definition.InitialRevision = func(*http.Request) uint64 { return session.currentRevision() }
	return definition, nil
}

func (s *ServerProgram[Bootstrap, Model, Msg]) validate() error {
	if s == nil || s.Store == nil {
		return fmt.Errorf("quicken: server program and store are required")
	}
	if s.ModelCodec.Encode == nil || s.ModelCodec.Decode == nil ||
		s.MessageCodec.Encode == nil || s.MessageCodec.Decode == nil {
		return fmt.Errorf("quicken: server program model and message codecs are required")
	}
	wire := browser.PlanToWire(s.Definition.Plan.Plan())
	if wire.Owner != "server" || wire.Transport != "live" {
		return fmt.Errorf("quicken: ServerProgram requires a server-owned live plan")
	}
	if s.Definition.SocketEndpoint == "" || s.Definition.PollEndpoint == "" ||
		s.Definition.EventEndpoint == "" {
		return fmt.Errorf("quicken: server program live endpoints are required")
	}
	if s.Security.ValidateOrigin == nil || s.Security.ValidateCSRF == nil {
		return fmt.Errorf("quicken: server program security validators are required")
	}
	return nil
}

func (s *ProgramSession[Model, Msg]) publish(requestID string, model Model) {
	encoded, err := s.codec.Encode(model)
	s.mu.Lock()
	s.revision++
	revision := s.revision
	if err == nil {
		s.model = append([]byte(nil), encoded...)
	}
	var envelope browser.LiveServerEnvelope
	if err != nil {
		envelope = browser.LiveServerEnvelope{
			ProtocolVersion: browser.LiveProtocolVersion,
			Type: browser.LiveErrorType,
			AppID: s.identity.AppID,
			ProgramID: s.identity.ProgramID,
			InstanceID: s.identity.InstanceID,
			RequestID: requestID,
			Revision: revision,
			Error: &browser.PublicError{
				Code: "model-encoding-failed",
				Message: "authoritative state could not be encoded",
			},
		}
	} else {
		envelope = browser.LiveServerEnvelope{
			ProtocolVersion: browser.LiveProtocolVersion,
			Type: browser.LiveSnapshotType,
			AppID: s.identity.AppID,
			ProgramID: s.identity.ProgramID,
			InstanceID: s.identity.InstanceID,
			RequestID: requestID,
			Revision: revision,
			Model: append([]byte(nil), encoded...),
		}
	}
	s.mu.Unlock()
	enqueueLatest(s.outbox, envelope)
}

func (s *ProgramSession[Model, Msg]) snapshot(requestID string) browser.LiveServerEnvelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	return browser.LiveServerEnvelope{
		ProtocolVersion: browser.LiveProtocolVersion,
		Type: browser.LiveSnapshotType,
		AppID: s.identity.AppID,
		ProgramID: s.identity.ProgramID,
		InstanceID: s.identity.InstanceID,
		RequestID: requestID,
		Revision: s.revision,
		Model: append([]byte(nil), s.model...),
	}
}

func (s *ProgramSession[Model, Msg]) currentRevision() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revision
}

func (s *ServerProgram[Bootstrap, Model, Msg]) dispatch(
	session *ProgramSession[Model, Msg],
	envelope browser.LiveClientEnvelope,
) error {
	if err := envelope.Validate(); err != nil {
		return err
	}
	if envelope.AppID != s.Definition.AppID ||
		envelope.ProgramID != s.Definition.ID ||
		envelope.InstanceID != session.identity.InstanceID {
		return fmt.Errorf("quicken: live event identity mismatch")
	}
	message, err := s.MessageCodec.Decode(envelope.Message)
	if err != nil {
		return err
	}
	session.runtime.Dispatch(message)
	return nil
}

func (s *ServerProgram[Bootstrap, Model, Msg]) socketHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if origin := req.Header.Get("Origin"); origin != "" &&
			s.Security.ValidateOrigin(req) != nil {
			http.Error(w, "origin rejected", http.StatusForbidden)
			return
		}
		conn, err := wsUpgrade(w, req)
		if err != nil {
			return
		}
		defer conn.Close()
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var resume browser.LiveClientEnvelope
		if json.Unmarshal(raw, &resume) != nil || resume.Type != browser.LiveResumeType ||
			resume.Validate() != nil {
			_ = writeLive(conn, liveError(s.Definition, resume.InstanceID, "", 0, "invalid-resume"))
			return
		}
		session, ok := s.Store.Get(resume.Token)
		if !ok {
			_ = writeLive(conn, liveError(s.Definition, resume.InstanceID, "", 0, "unknown-session"))
			return
		}
		drainProgramOutbox(session)
		if err := writeLive(conn, session.snapshot("")); err != nil {
			return
		}
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var event browser.LiveClientEnvelope
			if json.Unmarshal(raw, &event) != nil || event.Type != browser.LiveEventType {
				continue
			}
			if err := s.dispatch(session, event); err != nil {
				if writeLive(conn, liveError(
					s.Definition, session.identity.InstanceID,
					event.RequestID, session.currentRevision(), "invalid-event",
				)) != nil {
					return
				}
				continue
			}
			select {
			case update := <-session.outbox:
				if err := writeLive(conn, update); err != nil {
					return
				}
			case <-req.Context().Done():
				return
			}
		}
	})
}

func (s *ServerProgram[Bootstrap, Model, Msg]) pollHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if err := s.Security.ValidateOrigin(req); err != nil {
			http.Error(w, "origin rejected", http.StatusForbidden)
			return
		}
		token := req.URL.Query().Get("token")
		session, ok := s.Store.Get(token)
		if !ok {
			http.NotFound(w, req)
			return
		}
		revision := parseRevision(req.URL.Query().Get("revision"))
		if revision < session.currentRevision() {
			writeLiveHTTP(w, session.snapshot(""))
			return
		}
		timeout := s.PollTimeout
		if timeout <= 0 {
			timeout = 25 * time.Second
		}
		select {
		case update := <-session.outbox:
			writeLiveHTTP(w, update)
		case <-time.After(timeout):
			w.WriteHeader(http.StatusNoContent)
		case <-req.Context().Done():
			w.WriteHeader(http.StatusNoContent)
		}
	})
}

func (s *ServerProgram[Bootstrap, Model, Msg]) eventHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.Security.ValidateOrigin(req) != nil || s.Security.ValidateCSRF(req) != nil {
			http.Error(w, "request rejected", http.StatusForbidden)
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, req.Body, 1<<20))
		if err != nil {
			http.Error(w, "bad event", http.StatusBadRequest)
			return
		}
		var event browser.LiveClientEnvelope
		if json.Unmarshal(raw, &event) != nil || event.Type != browser.LiveEventType ||
			event.Validate() != nil {
			http.Error(w, "bad event", http.StatusBadRequest)
			return
		}
		session, ok := s.Store.Get(event.Token)
		if !ok {
			http.NotFound(w, req)
			return
		}
		if err := s.dispatch(session, event); err != nil {
			http.Error(w, "bad event", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func writeLive(conn *wsConn, envelope browser.LiveServerEnvelope) error {
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	return conn.WriteText(encoded)
}

func writeLiveHTTP(w http.ResponseWriter, envelope browser.LiveServerEnvelope) {
	encoded, err := json.Marshal(envelope)
	if err != nil {
		http.Error(w, "live encode failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(encoded)
}

func liveError[Bootstrap any, Model any, Msg any](
	definition Program[Bootstrap, Model, Msg],
	instanceID, requestID string,
	revision uint64,
	code string,
) browser.LiveServerEnvelope {
	return browser.LiveServerEnvelope{
		ProtocolVersion: browser.LiveProtocolVersion,
		Type: browser.LiveErrorType,
		AppID: definition.AppID,
		ProgramID: definition.ID,
		InstanceID: instanceID,
		RequestID: requestID,
		Revision: revision,
		Error: &browser.PublicError{Code: code, Message: "live request failed"},
	}
}

func enqueueLatest(channel chan browser.LiveServerEnvelope, envelope browser.LiveServerEnvelope) {
	select {
	case channel <- envelope:
	default:
		select {
		case <-channel:
		default:
		}
		select {
		case channel <- envelope:
		default:
		}
	}
}

func drainProgramOutbox[Model any, Msg any](session *ProgramSession[Model, Msg]) {
	for {
		select {
		case <-session.outbox:
		default:
			return
		}
	}
}

func parseRevision(value string) uint64 {
	var revision uint64
	_, _ = fmt.Sscan(value, &revision)
	return revision
}
