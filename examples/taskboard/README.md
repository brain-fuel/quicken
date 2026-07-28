# cadence-taskboard

A reference application proving one Cadence model, closed message algebra,
update function, codecs, commands, semantic view, and browser view across:

- Quicken full-page SSR and hydrated client ownership;
- Quicken SSR islands;
- Quicken server-owned live sessions;
- Bubble Tea v2 with Lip Gloss;
- Quicken Desktop on Windows, macOS, and Linux;
- Quicken Mobile on iOS and Android.

## Run

```sh
go run ./cmd/tui
go run ./cmd/desktop
go run ./cmd/server
```

Build browser assets into `web/`:

```sh
GOOS=js GOARCH=wasm go build -o web/taskboard.wasm ./cmd/wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/wasm_exec.js
cp ../../browser/client/cadence-loader.js web/cadence-loader.js
```

Then open `/` for a hydrated full page, `/island` for the composable region,
or `/live` for server-owned state.

Package `./cmd/mobile` with Gio's `gogio` tool for iOS or Android.

The taskboard is an integration example, not a separately published framework
dependency.

MIT, Copyright (c) 2026 Goforge.
