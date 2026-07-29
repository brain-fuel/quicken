# Cadence/Quicken UI family release validation

Release date: 2026-07-28

## Published modules

- `goforge.dev/cadence` v0.4.0
- `goforge.dev/quicken/web` v0.8.0
- `goforge.dev/quicken/tui` v0.1.0
- `goforge.dev/quicken/native` v0.1.0
- `goforge.dev/quicken/desktop` v0.1.0
- `goforge.dev/quicken/mobile` v0.1.1
- `goforge.dev/quicken/build` v0.1.0

## Passed gates

- deterministic Go+ generation checks for every module;
- unit, generated-law, and property tests;
- race detection;
- `go vet`;
- Quicken Web DOM and taskboard `GOOS=js GOARCH=wasm` builds;
- taskboard server, TUI, and Desktop builds;
- ForgeFlow WebAssembly, TUI, host desktop, and native arm64 iOS Simulator
  builds through the versioned Quicken build manifest;
- signed installation and launch on an iPhone 17 Pro Max simulator;
- Android emulator/device target diagnostics with the missing SDK tools
  enumerated;
- fresh external consumer compilation of every released module;
- no `replace` directive in any published module;
- public Go proxy resolution;
- goforge.dev production build with zero npm audit vulnerabilities;
- production landing-page, homepage-link, vanity-import, source, and license
  checks.

## External toolchain exception

The release host has Xcode 26.6 and the iOS 26.5 Simulator SDK, so native arm64
iOS Simulator packaging, signing, installation, and launch are verified. It
does not yet have an Android SDK/NDK. Android remains a first-class configured
target, but APK packaging is explicitly deferred and is not represented as
completed evidence.
