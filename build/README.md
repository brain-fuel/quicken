# Quicken Build

`goforge.dev/quicken/build` is the cross-platform build contract for Quicken
applications. It normalizes web, TUI, desktop, iOS, and Android target names
and keeps platform packaging out of application Taskfiles.

```sh
go run goforge.dev/quicken/build/cmd/quicken-build \
  -config quicken.yaml doctor all
go run goforge.dev/quicken/build/cmd/quicken-build \
  -config quicken.yaml run ios-simulator
```

iOS simulator packaging is implemented directly and distinguishes Apple
Silicon simulator `arm64` from device `arm64`. It does not invoke `gogio`.
Android is represented in the same target model and currently reports precise
SDK/NDK setup diagnostics until native APK packaging is enabled.

Version `v0.2.0` adds an independent `ios-device` path and explicit desktop
cross-toolchain configuration:

```yaml
targets:
  ios:
    minimum_version: "13.0"
    signing_identity: "Apple Development: Example"
    provisioning_profile: ./profiles/example.mobileprovision
  desktop:
    toolchains:
      windows-amd64:
        cc: x86_64-w64-mingw32-gcc
        cxx: x86_64-w64-mingw32-g++
```

MIT, Copyright (c) 2026 Goforge.
