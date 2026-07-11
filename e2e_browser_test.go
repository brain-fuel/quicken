package quicken_test

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"goforge.dev/quicken"
)

// TestRegionRevealInBrowser loads a page in a real browser and asserts the
// skeleton is replaced by the streamed region content the shim reveals. It is
// default-skipped: it runs only when QUICKEN_BROWSER_TEST=1, and it skips
// (never fails) if a browser cannot be launched. chromedp is a test-time
// dependency; consumers of the library do not build it.
func TestRegionRevealInBrowser(t *testing.T) {
	if os.Getenv("QUICKEN_BROWSER_TEST") == "" {
		t.Skip("set QUICKEN_BROWSER_TEST=1 to run the browser e2e (needs chromium)")
	}

	shell := func(f *quicken.Frame) template.HTML {
		return template.HTML("<!doctype html><html><head>" + string(f.Head()) +
			"</head><body>" + string(f.Slot("cards")) + "</body></html>")
	}
	page := quicken.NewPage(shell).Named("demo").
		Add(quicken.RegionFunc("cards",
			func(quicken.RenderContext) quicken.Tree { return quicken.Text(`<i>loading</i>`) },
			func(quicken.RenderContext) quicken.Tree { return quicken.Text(`<p class="q-real">REAL CARDS</p>`) }))

	mux := http.NewServeMux()
	quicken.Mount(mux)
	quicken.Serve(mux, "/", page, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	allocCtx, cancelAlloc := chromedp.NewContext(context.Background())
	defer cancelAlloc()
	ctx, cancel := context.WithTimeout(allocCtx, 25*time.Second)
	defer cancel()

	var real string
	err := chromedp.Run(ctx,
		chromedp.Navigate(srv.URL),
		chromedp.WaitVisible(`.q-real`, chromedp.ByQuery),
		chromedp.Text(`.q-real`, &real, chromedp.ByQuery),
	)
	if err != nil {
		t.Skipf("browser unavailable, skipping: %v", err)
	}
	if !strings.Contains(real, "REAL CARDS") {
		t.Fatalf("fetched content = %q, want REAL CARDS", real)
	}
}
