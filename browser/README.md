# Quicken Web Browser

The typed browser substrate inside `goforge.dev/quicken/web`.

- Typed node, attribute, event, and keyed patch values.
- Escaped SSR and versioned hydration manifests.
- No-replacement DOM hydration and delegated events.
- IME-aware controlled inputs and focus restoration.
- Typed remote commands with sequence, batch, delay, cancellation, and replay.
- Resumable WebSocket transport with long-poll fallback.

The portable root package has no `syscall/js` dependency. Browser globals are
isolated in `dom`; non-Wasm builds expose the same API and return
`dom.ErrUnavailable`.

```go
runtime, err := dom.Mount(dom.Options[Model, Msg]{
	Manifest: manifest,
	Logic: logic,
	View: view,
	Executor: dom.NewRemoteExecutor[Msg](manifest, fallback),
})
```

Application behavior is Go+. The JavaScript boundary is the standard
`wasm_exec.js` file plus `client/cadence-loader.js`, which discovers manifests
and starts the Wasm module.

MIT, Copyright (c) 2026 Goforge.
