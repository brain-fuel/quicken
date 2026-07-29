# Changelog

## v0.4.0

- Add host and Windows/macOS/Linux amd64/arm64 server artifact targets.
- Add Windows/macOS/Linux amd64/arm64 TUI artifact targets.
- Generate the complete server, TUI, and desktop architecture task matrix in
  every starter.

## v0.3.0

- Add `quicken-build init` for a generated full-stack Go+ starter.
- Generate Cadence logic, a shared semantic view, browser view, WebAssembly,
  server, TUI, desktop, mobile, Taskfile, and target manifest.
- Resolve the generic browser loader from the released Quicken Web module when
  applications do not provide a source path.
- Use platform-specific desktop, iOS, and Android semantic idioms in generated
  target adapters.

## v0.2.0

- Add separate `ios-device` compilation against `iphoneos`, optional
  provisioning/signing, and `devicectl` installation and launch.
- Configure non-host Gio desktop C/C++ compilers per operating system and
  architecture instead of disabling CGO.
- Diagnose missing cross compilers before invoking the Go build.

## v0.1.0

- Add the versioned Quicken application build manifest.
- Build and run WebAssembly, TUI, host desktop, and desktop architecture
  targets.
- Build, sign, install, and launch native arm64 iOS simulator applications
  without `gogio`.
- Represent Android emulator and device targets with actionable toolchain
  diagnostics while native packaging remains pending.
