# quicken

Fast-shell, deferred-region rendering for Go web apps. Paint the shell now,
fill the expensive parts as they become ready, stay readable without
JavaScript.

Status: phase 1 (core). Deferred first render over a streaming HTML transport.
Client-fetch, live updates, and more authoring adapters land in later phases.

License: MIT (Copyright (c) 2026 Goforge).

## Usage

```go
page := quicken.NewPage(func(f *quicken.Frame) template.HTML {
    return template.HTML("<!doctype html><html><head>" + string(f.Head()) +
        "</head><body>" + string(f.Slot("cards")) + "</body></html>")
}).Add(quicken.RegionFunc("cards",
    func(quicken.Context) quicken.Tree { return quicken.Text("<p>loading</p>") },
    func(quicken.Context) quicken.Tree { return quicken.Text(expensiveHTML()) },
))

mux := http.NewServeMux()
quicken.Mount(mux)                 // serves the swap shim
mux.Handle("/", page.Handler(nil)) // nil transport uses StreamHTML
```

With JavaScript the region is relocated into its slot as it finishes; without
JavaScript its content stays readable at the end of the document.
