# quicken

Fast-shell, deferred-region rendering for Go web apps. Paint the shell now,
fill the expensive parts as they become ready, stay readable without
JavaScript.

Status: phase 3. Deferred first render over a streaming HTML transport, a
ClientFetch transport with prefetch-on-intent, and a LiveChannel transport for
server-held live regions. More authoring adapters land in later phases.

License: MIT (Copyright (c) 2026 Goforge).

## Install

Requires Go 1.26 or newer.

```sh
go get github.com/brain-fuel/quicken
```

The library imports only the standard library; it pulls in no third-party
runtime dependencies.

## Usage

```go
page := quicken.NewPage(func(f *quicken.Frame) template.HTML {
    return template.HTML("<!doctype html><html><head>" + string(f.Head()) +
        "</head><body>" + string(f.Slot("cards")) + "</body></html>")
}).Add(quicken.RegionFunc("cards",
    func(quicken.RenderContext) quicken.Tree { return quicken.Text("<p>loading</p>") },
    func(quicken.RenderContext) quicken.Tree { return quicken.Text(expensiveHTML()) },
))

mux := http.NewServeMux()
quicken.Mount(mux)                 // serves the swap shim
mux.Handle("/", page.Handler(nil)) // nil transport uses StreamHTML
```

With JavaScript the region is relocated into its slot as it finishes; without
JavaScript its content stays readable at the end of the document.

## ClientFetch and prefetch

The ClientFetch transport sends a fast shell with skeletons and no region
content; the client fetches each region from `/_regions/<page>/<id>` after
load. Mount it with Serve so those endpoints exist:

ClientFetch is a JavaScript enhancement: with scripting disabled the page
shows only skeletons. Use the default StreamHTML transport when a
no-JavaScript content floor is required.

```go
mux := http.NewServeMux()
quicken.Mount(mux)
quicken.Serve(mux, "/", page.Named("demo"), quicken.ClientFetch{})
```

Prefetch-on-intent warms the client cache before a click. Mark a trigger
element with `data-q-prefetch="/_regions/demo/cards"` and, optionally,
`data-q-prefetch-on="mouseover"` (the default), `focusin`, or `visible`. The
cache is shared with region fetches, so a prefetched url loads instantly.

## Live regions

`AddLive` registers a region that keeps server-held state across events,
instead of the one-shot `Render(ctx)` a deferred `Region` uses. Mount it with
`Serve` and the `LiveChannel{}` transport:

```go
page := quicken.NewPage(shell).Named("demo").
    AddLive(myCounter{})

mux := http.NewServeMux()
quicken.Mount(mux)
quicken.Serve(mux, "/", page, quicken.LiveChannel{})
```

`myCounter` implements `LiveRegion`: `Mount` produces the initial `State`,
`HandleEvent` applies a named client event to that state, and `Render(State)`
produces the region's `Tree`. The page's shell marks up an event source with
`data-live-click="<event name>"` (or another `data-live-*` binding) inside the
region's slot; a click on it sends the named event to the server, which
applies it, diffs the result against what the client already has, and pushes
back only the dynamic slots that changed.

The client opens a WebSocket to the server and carries a resume token (minted
on first load and embedded in the page) so a reconnect reattaches to the same
session's state rather than remounting. When a WebSocket cannot be opened, or
one drops, the client falls back to an HTTP long-poll transport automatically,
using the same token and message shapes.

LiveChannel is a JavaScript enhancement: with scripting off, a live region
shows its skeleton and nothing more, since there is no socket to carry state
or events. When a no-JavaScript content floor is required, use the default
StreamHTML transport instead.

### Production notes

Two limitations apply to LiveChannel in this release:

- The built-in in-memory session store never evicts a session, so every page
  load adds a `LiveSession` that lives for the process lifetime. This is fine
  for development and small deployments, but for production supply a bounded
  `SessionStore` (the interface is the seam for a TTL or LRU store). A tracked
  follow-up will add an evicting default.
- The WebSocket upgrade does not check the `Origin` header. An application that
  needs cross-site request protection should validate `Origin` itself before
  upgrading. Note that driving a live region already requires the unguessable
  per-session resume token embedded in the page, so an event cannot be injected
  without first reading that page.

## Testing

The library code is standard-library only; chromedp is a test-time dependency
(imported only by the browser test), so consumers of the library never build
it. The headless-browser end-to-end test is part of the normal suite and is
default-skipped. Run it opt-in:

```
QUICKEN_BROWSER_TEST=1 go test ./...
```

It skips cleanly when no browser is installed.
