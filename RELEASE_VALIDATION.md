# Cadence/Quicken UI family release validation

Release date: 2026-07-28

## Published modules

- `goforge.dev/cadence` v0.4.0
- `goforge.dev/quicken/web` v0.8.0
- `goforge.dev/quicken/tui` v0.1.0
- `goforge.dev/quicken/native` v0.1.0
- `goforge.dev/quicken/desktop` v0.1.0
- `goforge.dev/quicken/mobile` v0.1.0

## Passed gates

- deterministic Go+ generation checks for every module;
- unit, generated-law, and property tests;
- race detection;
- `go vet`;
- Quicken Web DOM and taskboard `GOOS=js GOARCH=wasm` builds;
- taskboard server, TUI, and Desktop builds;
- fresh external consumer compilation of every released module;
- no `replace` directive in any published module;
- public Go proxy resolution;
- goforge.dev production build with zero npm audit vulnerabilities;
- production landing-page, homepage-link, vanity-import, source, and license
  checks.

## External toolchain exception

The release host was Darwin/arm64 with Command Line Tools only. It did not have
a full Xcode installation or an Android SDK/NDK, so signed iOS and Android
`gogio` packaging could not be executed locally. Quicken Mobile's Go+ generation,
host compilation, unit/vet/race gates, clean-module consumption, platform build
tags, navigation semantics, environment model, and typed mobile effects all
passed. Device packaging remains a platform-toolchain integration gate and is
not represented as completed evidence.
