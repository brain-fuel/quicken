# ForgeFlow

ForgeFlow is a non-trivial cross-target incident-response and field-operations
application. One Go+ Cadence model, closed message algebra, update function,
effect boundary, codecs, and semantic view deploy through Quicken to:

- server-rendered and hydrated WebAssembly browser UI;
- server-owned live browser sessions;
- terminal UI using Bubble Tea and Lip Gloss;
- native Windows, macOS, and Linux desktop applications;
- native iOS and Android mobile applications.

The example implements incident creation, severity and lifecycle management,
nested response tasks, an append-only operational timeline, full-text
filtering, optimistic persistence, online/offline transitions, queued
synchronization, and explicit conflict resolution.

## Run locally

```sh
go run ./cmd/tui
go run ./cmd/desktop
go run ./cmd/server
```

Build browser assets into `web/`:

```sh
mkdir -p web
GOOS=js GOARCH=wasm go build -o web/forgeflow.wasm ./cmd/wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/wasm_exec.js
cp ../../browser/client/cadence-loader.js web/cadence-loader.js
go run ./cmd/server
```

Open `/` for hydrated ownership, `/island` for a composable region, or `/live`
for server-owned state.

## Package native targets

```sh
go build -o dist/forgeflow-tui ./cmd/tui
go build -o dist/forgeflow-desktop ./cmd/desktop
gogio -target android -appid dev.goforge.forgeflow ./cmd/mobile
gogio -target ios -appid dev.goforge.forgeflow ./cmd/mobile
```

Android packaging requires the Android SDK/NDK. iOS packaging requires macOS
and Xcode. Desktop cross-packaging follows Gio's platform toolchain
requirements.

The module path remains `goforge.dev/cadence-taskboard` so existing release
examples continue to resolve; ForgeFlow is the application identity.

MIT, Copyright (c) 2026 Goforge.
