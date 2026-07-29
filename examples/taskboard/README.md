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
task run:tui
task run:desktop
task run:web
task run:ios
task run:android
```

The project uses GoForge Task with `quicken.yaml` as the only application build
manifest. Check all configured toolchains with:

```sh
task doctor
```

Open `/` for hydrated ownership, `/island` for a composable region, or `/live`
for server-owned state.

## Package native targets

```sh
task build:available
task build:desktop:windows:amd64
task build:desktop:windows:arm64
task build:desktop:macos:amd64
task build:desktop:macos:arm64
task build:desktop:linux:amd64
task build:desktop:linux:arm64
```

Android packaging requires the Android SDK/NDK. iOS packaging requires macOS
and Xcode. `task run:ios` builds the correct simulator architecture, signs,
installs, and launches the application. Until the Android SDK/NDK is installed,
`task doctor:android` and `task run:android` preserve the configured target and
report the exact missing toolchain instead of silently omitting Android.

The module path remains `goforge.dev/cadence-taskboard` so existing release
examples continue to resolve; ForgeFlow is the application identity.

MIT, Copyright (c) 2026 Goforge.
