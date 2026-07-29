# Changelog

## v0.2.0

- Add a semantic mobile adapter that selects iOS or Android interaction idioms.
- Apply environment safe-area insets without forking application views.
- Enforce 44 dp iOS and 48 dp Android minimum interactive-control heights.

## v0.1.1

- Generate valid iOS and Android platform variants from build-tagged Go+
  source files.
- Keep closed platform-enum construction in the untagged semantic unit so
  target builds remain deterministic across Go+ generation.

## v0.1.0

- Initial iOS and Android application facade.
- Environment, safe-area, lifecycle, and platform values.
- Immutable navigation stack.
- Typed permission, share, haptic, and location effect protocol.
