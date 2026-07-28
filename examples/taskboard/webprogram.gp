package taskboard

import (
	"net/http"

	"goforge.dev/cadence"
	"goforge.dev/cadence/program"
	browser "goforge.dev/quicken/web/browser"
	"goforge.dev/quicken/web"
)

const (
	AppID          = "forgeflow"
	HydratedID     = "forgeflow-hydrated"
	LiveID         = "forgeflow-live"
	CommandPath    = "/_forgeflow/command"
	LiveSocketPath = "/_forgeflow/live/socket"
	LivePollPath   = "/_forgeflow/live/poll"
	LiveEventPath  = "/_forgeflow/live/event"
)

func WebProgram(id string, plan cadence.ValidatedPlan, save SaveEffect) quicken.Program[Bootstrap, Model, Msg] {
	return quicken.Program[Bootstrap, Model, Msg]{
		AppID: AppID,
		ID: id,
		Plan: plan,
		Assets: quicken.BrowserAssets{
			Hash: "development",
			WasmURL: "/assets/forgeflow.wasm",
			WasmExecURL: "/assets/wasm_exec.js",
			LoaderURL: "/assets/cadence-loader.js",
		},
		CommandEndpoint: CommandPath,
		SocketEndpoint: LiveSocketPath,
		PollEndpoint: LivePollPath,
		EventEndpoint: LiveEventPath,
		DocumentTitle: "ForgeFlow Operations",
		Bootstrap: func(*http.Request) (Bootstrap, error) {
			return Bootstrap{Initial: InitialModel()}, nil
		},
		Logic: func(bootstrap Bootstrap) program.Logic[Model, Msg] {
			return Logic(bootstrap.Initial, save)
		},
		View: BrowserView,
		Skeleton: func(Bootstrap) browser.Node[Msg] {
			return browser.Element[Msg]("p", browser.Text[Msg]("Loading ForgeFlow..."))
		},
		NoScript: func(bootstrap Bootstrap) browser.Node[Msg] {
			return BrowserView(bootstrap.Initial)
		},
	}
}

func HydratedProgram() quicken.Program[Bootstrap, Model, Msg] {
	return WebProgram(HydratedID, cadence.Hydrated(cadence.ActivateLoad()), RemoteSave)
}

func LiveProgram() quicken.Program[Bootstrap, Model, Msg] {
	return WebProgram(LiveID, cadence.LiveServer(cadence.ActivateLoad()), LocalSave)
}
