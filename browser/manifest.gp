package browser

import (
	"encoding/json"
	"fmt"

	"goforge.dev/cadence"
)

const ProtocolVersion = 1

const (
	ScopeFullPage = "full-page"
	ScopeIsland   = "island"
)

type PlanWire struct {
	Initial    string `json:"initial"`
	Owner      string `json:"owner"`
	Activation string `json:"activation"`
	Transport  string `json:"transport"`
	Fallback   string `json:"fallback"`
}

type Endpoints struct {
	Command  string `json:"command,omitempty"`
	Socket   string `json:"socket,omitempty"`
	LongPoll string `json:"long_poll,omitempty"`
	Event    string `json:"event,omitempty"`
}

type Manifest struct {
	ProtocolVersion int             `json:"protocol_version"`
	AssetHash       string          `json:"asset_hash"`
	AppID           string          `json:"app_id"`
	ProgramID       string          `json:"program_id"`
	InstanceID      string          `json:"instance_id"`
	MountID         string          `json:"mount_id"`
	Scope           string          `json:"scope"`
	Plan            PlanWire        `json:"plan"`
	Bootstrap       json.RawMessage `json:"bootstrap"`
	CommandCodec    int             `json:"command_codec"`
	InitialRevision uint64          `json:"initial_revision"`
	Endpoints       Endpoints       `json:"endpoints"`
	ResumeToken     string          `json:"resume_token,omitempty"`
}

func NewManifest(
	assetHash, appID, programID, instanceID, mountID, scope string,
	plan cadence.ValidatedPlan,
	bootstrap json.RawMessage,
) Manifest {
	return Manifest{
		ProtocolVersion: ProtocolVersion,
		AssetHash: assetHash, AppID: appID, ProgramID: programID,
		InstanceID: instanceID, MountID: mountID, Scope: scope,
		Plan: PlanToWire(plan.Plan()), Bootstrap: cloneBytes(bootstrap),
		CommandCodec: 1,
	}
}

func (m Manifest) Validate() error {
	if m.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("quicken/web/browser: unsupported manifest protocol %d", m.ProtocolVersion)
	}
	if m.AssetHash == "" || m.AppID == "" || m.ProgramID == "" ||
		m.InstanceID == "" || m.MountID == "" {
		return fmt.Errorf("quicken/web/browser: manifest identity fields must be non-empty")
	}
	if m.Scope != ScopeFullPage && m.Scope != ScopeIsland {
		return fmt.Errorf("quicken/web/browser: invalid mount scope %q", m.Scope)
	}
	if m.CommandCodec != 1 {
		return fmt.Errorf("quicken/web/browser: unsupported command codec %d", m.CommandCodec)
	}
	if _, err := PlanFromWire(m.Plan); err != nil {
		return err
	}
	if !json.Valid(m.Bootstrap) {
		return fmt.Errorf("quicken/web/browser: bootstrap must be valid JSON")
	}
	return nil
}

func (m Manifest) Encode() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

func DecodeManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	manifest.Bootstrap = cloneBytes(manifest.Bootstrap)
	return manifest, nil
}

func PlanToWire(plan cadence.Plan) PlanWire {
	return PlanWire{
		Initial: initialName(plan.Initial),
		Owner: ownerName(plan.Owner),
		Activation: activationName(plan.Activation),
		Transport: transportName(plan.Transport),
		Fallback: fallbackName(plan.Fallback),
	}
}

func PlanFromWire(wire PlanWire) (cadence.ValidatedPlan, error) {
	initial, err := parseInitial(wire.Initial)
	if err != nil {
		return cadence.ValidatedPlan{}, err
	}
	owner, err := parseOwner(wire.Owner)
	if err != nil {
		return cadence.ValidatedPlan{}, err
	}
	activation, err := parseActivation(wire.Activation)
	if err != nil {
		return cadence.ValidatedPlan{}, err
	}
	transport, err := parseTransport(wire.Transport)
	if err != nil {
		return cadence.ValidatedPlan{}, err
	}
	fallback, err := parseFallback(wire.Fallback)
	if err != nil {
		return cadence.ValidatedPlan{}, err
	}
	return cadence.Validate(cadence.Plan{
		Initial: initial, Owner: owner, Activation: activation,
		Transport: transport, Fallback: fallback,
	})
}

func initialName(value cadence.InitialRender) string {
	return cadence.InitialRenderFold(value, cadence.InitialRenderCases[string]{
		InitialFull: func() string { return "full" },
		InitialSkeleton: func() string { return "skeleton" },
		InitialNone: func() string { return "none" },
	})
}

func ownerName(value cadence.StateOwner) string {
	return cadence.StateOwnerFold(value, cadence.StateOwnerCases[string]{
		OwnerNone: func() string { return "none" },
		OwnerServer: func() string { return "server" },
		OwnerClient: func() string { return "client" },
	})
}

func activationName(value cadence.Activation) string {
	return cadence.ActivationFold(value, cadence.ActivationCases[string]{
		ActivateRequest: func() string { return "request" },
		ActivateLoad: func() string { return "load" },
		ActivateVisible: func() string { return "visible" },
		ActivateIntent: func() string { return "intent" },
	})
}

func transportName(value cadence.Transport) string {
	return cadence.TransportFold(value, cadence.TransportCases[string]{
		TransportNone: func() string { return "none" },
		TransportFetch: func() string { return "fetch" },
		TransportStream: func() string { return "stream" },
		TransportLive: func() string { return "live" },
	})
}

func fallbackName(value cadence.Fallback) string {
	return cadence.FallbackFold(value, cadence.FallbackCases[string]{
		FallbackUniversalFloor: func() string { return "universal-floor" },
		FallbackStaticSnapshot: func() string { return "static-snapshot" },
		FallbackRequiresClient: func() string { return "requires-client" },
	})
}

func parseInitial(value string) (cadence.InitialRender, error) {
	switch value {
	case "full":
		return cadence.InitialFull(), nil
	case "skeleton":
		return cadence.InitialSkeleton(), nil
	case "none":
		return cadence.InitialNone(), nil
	default:
		return nil, fmt.Errorf("quicken/web/browser: invalid initial render %q", value)
	}
}

func parseOwner(value string) (cadence.StateOwner, error) {
	switch value {
	case "none":
		return cadence.OwnerNone(), nil
	case "server":
		return cadence.OwnerServer(), nil
	case "client":
		return cadence.OwnerClient(), nil
	default:
		return nil, fmt.Errorf("quicken/web/browser: invalid state owner %q", value)
	}
}

func parseActivation(value string) (cadence.Activation, error) {
	switch value {
	case "request":
		return cadence.ActivateRequest(), nil
	case "load":
		return cadence.ActivateLoad(), nil
	case "visible":
		return cadence.ActivateVisible(), nil
	case "intent":
		return cadence.ActivateIntent(), nil
	default:
		return nil, fmt.Errorf("quicken/web/browser: invalid activation %q", value)
	}
}

func parseTransport(value string) (cadence.Transport, error) {
	switch value {
	case "none":
		return cadence.TransportNone(), nil
	case "fetch":
		return cadence.TransportFetch(), nil
	case "stream":
		return cadence.TransportStream(), nil
	case "live":
		return cadence.TransportLive(), nil
	default:
		return nil, fmt.Errorf("quicken/web/browser: invalid transport %q", value)
	}
}

func parseFallback(value string) (cadence.Fallback, error) {
	switch value {
	case "universal-floor":
		return cadence.FallbackUniversalFloor(), nil
	case "static-snapshot":
		return cadence.FallbackStaticSnapshot(), nil
	case "requires-client":
		return cadence.FallbackRequiresClient(), nil
	default:
		return nil, fmt.Errorf("quicken/web/browser: invalid fallback %q", value)
	}
}

func cloneBytes(data []byte) []byte {
	return append([]byte(nil), data...)
}
