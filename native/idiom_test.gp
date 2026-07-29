package native

import (
	"testing"
	"testing/quick"
)

func TestPlatformIdiomControlSizeLaw(t *testing.T) {
	law := func(_ uint8) bool {
		desktop := DesktopIdiom()
		ios := IOSIdiom()
		android := AndroidIdiom()
		return desktop.MinimumControlHeight > 0 &&
			desktop.MinimumControlHeight < ios.MinimumControlHeight &&
			ios.MinimumControlHeight < android.MinimumControlHeight &&
			desktop.Gap <= ios.Gap && ios.Gap == android.Gap
	}
	if err := quick.Check(law, nil); err != nil {
		t.Fatal(err)
	}
}
