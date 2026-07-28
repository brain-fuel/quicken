# Changelog

## v0.8.0

- Consolidate the staged browser interpreter under `quicken/web/browser`.
- Add isomorphic `Program` definitions for shared server and browser logic.
- Add full-page and composable-region SSR with versioned hydration manifests.
- Add immutable browser asset and security configuration.
- Add typed remote command registration, CSRF/origin hooks, request limits, and
  idempotent replay.
- Add authoritative server-owned programs with model/message codecs, revisioned
  sessions, WebSocket resume, and long-poll fallback.
- Retain the earlier `Page`/`Serve` API during the migration window.
