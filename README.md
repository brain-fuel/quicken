# quicken

Quicken is the Web, TUI, desktop, and mobile UI family for Cadence programs.
Applications share an Elm-style model, message algebra, update function,
effects, and semantic view while target interpreters preserve native platform
idioms.

## Cross-platform builds

`goforge.dev/quicken/build` owns the versioned application manifest and target
toolchain boundary. GoForge Task supplies the uniform command surface:

```sh
task doctor
task build:web
task build:desktop
task run:ios
task run:android
```

The iOS simulator path distinguishes simulator `arm64` from device `arm64` and
does not invoke `gogio`. Android remains explicitly configured and reports
missing SDK/NDK tools until the Android toolchain is installed.

## Quicken Web

`goforge.dev/quicken/web` is the HTTP, DOM, and server-state interpreter for
Cadence programs. It provides full-page and island SSR, versioned hydration
manifests, typed remote commands, and authoritative live sessions over
WebSocket with long-poll fallback.

## Isomorphic programs

```go
definition := quicken.Program[Bootstrap, Model, Msg]{
	AppID: "tasks",
	ID: "task-list",
	Plan: cadence.Hydrated(cadence.ActivateLoad{}),
	Assets: assets,
	Bootstrap: loadPublicState,
	Logic: logicFromBootstrap,
	View: browserView,
	NoScript: fallbackView,
}

mux.Handle("/", quicken.FullPageProgram[Bootstrap, Model, Msg]{
	MountID: "app",
	Program: definition,
})
```

`ProgramRegion` renders the same definition as a composable island.
`NewServerProgram` changes ownership to the server and mounts revisioned
socket, poll, and event routes.

## Browser boundary

Application behavior remains Go+. The page loads the standard Go Wasm support
file and a small generic loader. `goforge.dev/quicken/web/browser/dom` performs
hydration, keyed patches, delegated events, controlled-input reconciliation,
remote command execution, and live synchronization.

## Commands and security

`RegisterCommand` binds concrete request and response types. `CommandEndpoint`
checks protocol versions, request size, configured origin/CSRF policy, and a
replay store keyed by instance/request ID. Public errors are explicit and do
not expose internal error details.

## Compatibility path

The earlier region `Page`/`Serve` API remains for the v0.7 transition. New
applications should use `Program`, `FullPageProgram`, `ProgramRegion`, and
`ServerProgram`.

## License

MIT, Copyright (c) 2026 Goforge.
