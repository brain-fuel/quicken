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

MIT, Copyright (c) 2026 Goforge.
