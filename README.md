# quicken

Fast-shell, deferred-region rendering for Go web apps. Paint the shell now,
fill the expensive parts as they become ready, stay readable without
JavaScript.

Status: SP2. One page entry point, `Serve(mux, path, page, policy)`: it streams
a universal content floor (shell and skeletons first, then every region's full
content) and a `cadence.Policy` decides, per region, how the client reveals
that content (eager, on load, on visibility, or on hover). Live regions keep
server-held state over a WebSocket (with an HTTP long-poll fallback). The
markup marker document and html/template helper authoring adapters have landed;
explorer integration remains for a later phase.

Because the floor always ships every region's real content, SP2 governs *when
and how the client reveals* that content, not *whether the server computes it*:
first paint is fast, but the server still renders every region. True
compute-skip (never rendering an off-screen region) requires a client that
computes from shipped data, and lands with the `cadence` TEA interpreter in a
later phase (SP3) as an explicit opt-out of the no-JS floor.

License: MIT (Copyright (c) 2026 Goforge).

## Install

Requires Go 1.26 or newer.

```sh
go get goforge.dev/quicken
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
quicken.Mount(mux)               // serves the shim
quicken.Serve(mux, "/", page, nil) // nil policy: kind-inferred defaults
```

The floor always carries every region's real content, so with JavaScript off
the page is fully readable. With JavaScript the shim reveals each region into
its slot according to the policy.

## Reveal strategies (cadence.Policy)

A `nil` policy uses the kind-inferred default: plain regions are deferred and
revealed on load, live regions go live. Pass a `cadence.Policy` to choose a
different reveal strategy per region. Strategies come from the `cadence`
package: `Eager` (reveal immediately), `Deferred{Server, OnLoad|OnVisible|OnHover}`,
and `Live`.

```go
import "goforge.dev/cadence"

pol := cadence.Fixed(map[string]cadence.Strategy{
    "hero":  {Kind: cadence.Eager},
    "cards": {Kind: cadence.Deferred, Where: cadence.Server, On: cadence.OnVisible},
})

mux := http.NewServeMux()
quicken.Mount(mux)
quicken.Serve(mux, "/", page.Named("demo"), pol)
```

`OnVisible` defers the reveal behind an `IntersectionObserver` on the slot;
`OnHover` defers it behind a mouseover/focusin listener. Because the content
is already in the floor, these are pure client-side reveal timings, not extra
round trips: with scripting off the content is visible regardless.

## Live regions

`AddLive` registers a region that keeps server-held state across events,
instead of the one-shot `Render(ctx)` a deferred `Region` uses. Serve mounts
its WebSocket and long-poll routes automatically; a nil policy sends live
regions live, or set them explicitly with a policy:

```go
page := quicken.NewPage(shell).Named("demo").
    AddLive(myCounter{})

mux := http.NewServeMux()
quicken.Mount(mux)
quicken.Serve(mux, "/", page, nil) // live regions inferred as Live
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

The floor streams each live region's first render as its no-JavaScript
snapshot, so a live region is readable with scripting off; the socket only
adds the live updates on top. On load the shim swaps that snapshot into the
slot before opening the socket, so there is no skeleton flash.

### Production notes

Two limitations apply to live regions in this release:

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

## Authoring

A page's shell can be authored three ways, and all three produce an ordinary
`*Page`, so `Serve` serves them identically.

- **Func-registry.** Write the shell as a Go function over a `*Frame`, calling
  `f.Head()` and `f.Slot(id)` where the shim and each region belong. This is
  the style shown in Usage above, and it gives full control since the shell is
  plain Go.
- **Marker document.** Write the shell as an HTML string carrying quicken
  markers, and hand it to `FromMarkup`. `<!--quicken head-->` marks the shim,
  and `<!--quicken lazy id-->` or `<!--quicken live id-->` marks a region's
  slot. Register the regions with `Add` and `AddLive` as usual; the slot each
  marker produces follows the registration.

  ```go
  page := quicken.FromMarkup(`<!doctype html><html>
  <head><!--quicken head--></head>
  <body><!--quicken lazy cards--></body></html>`).
      Add(cardsRegion)
  ```

- **Template helpers.** Author the shell as an `html/template`, using the
  `lazy`, `live`, and `quickenHead` funcs from `Helpers()` to emit the same
  markers a marker document would. Render the template with `RenderMarkup`,
  then hand the result to `FromMarkup`.

  ```go
  tmpl := template.Must(template.New("page").Funcs(quicken.Helpers()).Parse(page))
  markup, _ := quicken.RenderMarkup(tmpl, data)
  page := quicken.FromMarkup(markup).Add(cardsRegion)
  ```

  A literal `<!--quicken ...-->` comment hand-typed directly into an
  html/template source is stripped by html/template and produces nothing, so
  an author working in html/template must use the `Helpers()` funcs (which
  return `template.HTML`) rather than typing the marker comment by hand; a
  marker document passed straight to `FromMarkup` (not routed through
  html/template) is unaffected by this and can contain the literal comment.

Because the marker document and template-helper styles both go through
`FromMarkup`, and `FromMarkup` builds an ordinary func-registry `*Page`
underneath, all three styles are interchangeable: switching how a shell is
authored never changes what gets delivered.

## Testing

The library code is standard-library only; chromedp is a test-time dependency
(imported only by the browser test), so consumers of the library never build
it. The headless-browser end-to-end test is part of the normal suite and is
default-skipped. Run it opt-in:

```
QUICKEN_BROWSER_TEST=1 go test ./...
```

It skips cleanly when no browser is installed.
