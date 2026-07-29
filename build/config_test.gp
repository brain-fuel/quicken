package build

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
)

func TestLoadDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quicken.yaml")
	err := os.WriteFile(path, []byte(`version: 1
application:
  name: Example
  identifier: dev.goforge.example
commands:
  mobile: ./cmd/mobile
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Application.Version != "0.1.0" || config.Application.Build != 1 {
		t.Fatalf("unexpected application defaults: %#v", config.Application)
	}
	if config.Targets.IOS.MinimumVersion != "13.0" {
		t.Fatalf("unexpected iOS minimum: %q", config.Targets.IOS.MinimumVersion)
	}
	if config.Targets.Android.MinimumSDK != 26 || config.Targets.Android.TargetSDK != 36 {
		t.Fatalf("unexpected Android defaults: %#v", config.Targets.Android)
	}
}

func TestIOSSimulatorPlistIdentityLaw(t *testing.T) {
	law := func(build uint16) bool {
		config := Config{
			Application: Application{
				Name: "LawApp", Identifier: "dev.goforge.law",
				Version: "1.2.3", Build: int(build) + 1,
			},
			Targets: Targets{IOS: IOS{MinimumVersion: "13.0"}},
		}
		plist := iosSimulatorPlist(config)
		return strings.Contains(plist, "<string>iPhoneSimulator</string>") &&
			strings.Contains(plist, "<string>dev.goforge.law</string>") &&
			strings.Contains(plist, "<string>"+strconv.Itoa(int(build)+1)+"</string>")
	}
	if err := quick.Check(law, nil); err != nil {
		t.Fatal(err)
	}
}
