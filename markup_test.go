package quicken

import "testing"

func TestParseMarkupLiteralOnly(t *testing.T) {
	segs, err := parseMarkup("<p>hello</p>")
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 1 || segs[0].kind != kindLiteral || segs[0].text != "<p>hello</p>" {
		t.Fatalf("segs = %+v", segs)
	}
}

func TestParseMarkupLazyAndLive(t *testing.T) {
	segs, err := parseMarkup(`<a><!--quicken lazy cards--><b><!--quicken live counter--></a>`)
	if err != nil {
		t.Fatal(err)
	}
	want := []markupSeg{
		{kindLiteral, "<a>"},
		{kindLazy, "cards"},
		{kindLiteral, "<b>"},
		{kindLive, "counter"},
		{kindLiteral, "</a>"},
	}
	if len(segs) != len(want) {
		t.Fatalf("len = %d, want %d (%+v)", len(segs), len(want), segs)
	}
	for i := range want {
		if segs[i] != want[i] {
			t.Fatalf("seg %d = %+v, want %+v", i, segs[i], want[i])
		}
	}
}

func TestParseMarkupHead(t *testing.T) {
	segs, err := parseMarkup(`<head><!--quicken head--></head>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 3 || segs[1].kind != kindHead || segs[1].text != "" {
		t.Fatalf("segs = %+v", segs)
	}
}

func TestParseMarkupTrailingLiteral(t *testing.T) {
	segs, err := parseMarkup(`<!--quicken lazy x-->tail`)
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 2 || segs[0].kind != kindLazy || segs[1].kind != kindLiteral || segs[1].text != "tail" {
		t.Fatalf("segs = %+v", segs)
	}
}

func TestParseMarkupRejectsUnknownDirective(t *testing.T) {
	if _, err := parseMarkup(`<!--quicken frobnicate x-->`); err == nil {
		t.Fatal("expected error on unknown directive")
	}
}

func TestParseMarkupRejectsInvalidID(t *testing.T) {
	if _, err := parseMarkup(`<!--quicken lazy bad/id-->`); err == nil {
		t.Fatal("expected error on invalid id")
	}
}

func TestParseMarkupRejectsMissingID(t *testing.T) {
	if _, err := parseMarkup(`<!--quicken lazy-->`); err == nil {
		t.Fatal("expected error on missing id")
	}
}

func TestParseMarkupRejectsHeadWithArg(t *testing.T) {
	if _, err := parseMarkup(`<!--quicken head extra-->`); err == nil {
		t.Fatal("expected error on head with an argument")
	}
}

func TestParseMarkupRejectsEmptyDirective(t *testing.T) {
	if _, err := parseMarkup(`<!--quicken -->`); err == nil {
		t.Fatal("expected error on empty marker directive")
	}
}

func TestParseMarkupUnterminatedMarkerIsLiteral(t *testing.T) {
	// A sentinel with no closing --> is treated as ordinary literal text, not
	// an error, so arbitrary documents that happen to contain the prefix do not
	// break.
	segs, err := parseMarkup(`<!--quicken lazy x`)
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 1 || segs[0].kind != kindLiteral {
		t.Fatalf("segs = %+v", segs)
	}
}
