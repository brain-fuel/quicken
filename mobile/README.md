# Quicken Mobile

`goforge.dev/quicken/mobile` runs Cadence programs as native iOS and Android
applications through Gio and Quicken Native.

It adds:

- iOS/Android platform identity and single-window lifecycle;
- environment and safe-area input to views;
- persistent functional navigation stacks;
- typed permission, share, haptic, and location effect names;
- application bundle identifiers.

```go
model, err := mobile.Run(mobile.Application[Model, Msg]{
	ID: "dev.goforge.tasks",
	Logic: logic,
	View: mobileView,
	Effects: platformEffects,
})
```

Package with Gio's `gogio` command for Android or iOS. Platform permission
handlers remain explicit application capabilities rather than hidden globals.

MIT, Copyright (c) 2026 Goforge.
