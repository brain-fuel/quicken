package build

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Options struct {
	Root   string
	Dist   string
	Device string
}

func Run(config Config, action, target string, options Options) error {
	if options.Root == "" {
		options.Root = "."
	}
	if options.Dist == "" {
		options.Dist = filepath.Join(options.Root, "dist")
	}
	switch action {
	case "doctor":
		return doctor(config, target)
	case "build":
		return buildTarget(config, target, options)
	case "run":
		return runTarget(config, target, options)
	default:
		return fmt.Errorf("unknown action %q; expected doctor, build, or run", action)
	}
}

func buildTarget(config Config, target string, options Options) error {
	if err := os.MkdirAll(options.Dist, 0o755); err != nil {
		return err
	}
	switch target {
	case "web":
		return buildWeb(config, options)
	case "tui":
		return goBuild(config.Commands.TUI, filepath.Join(options.Dist, artifactName(config, "tui")), nil)
	case "desktop":
		return goBuild(config.Commands.Desktop, filepath.Join(options.Dist, artifactName(config, "desktop")), nil)
	case "ios-simulator":
		return buildIOSSimulator(config, options)
	case "ios-device":
		return buildIOSDevice(config, options)
	case "android-emulator", "android-device":
		return androidUnavailable(config)
	default:
		return buildDesktopCross(config, target, options)
	}
}

func runTarget(config Config, target string, options Options) error {
	switch target {
	case "web":
		if err := buildWeb(config, options); err != nil {
			return err
		}
		return foreground(config.Commands.Server, nil)
	case "tui":
		return foreground(config.Commands.TUI, nil)
	case "desktop":
		return foreground(config.Commands.Desktop, nil)
	case "ios", "ios-simulator":
		if err := buildIOSSimulator(config, options); err != nil {
			return err
		}
		app := filepath.Join(options.Dist, config.Application.Name+".app")
		if _, err := command("codesign", "--force", "--sign", "-", app); err != nil {
			return err
		}
		if _, err := command("xcrun", "simctl", "install", "booted", app); err != nil {
			return fmt.Errorf("install iOS simulator application: %w", err)
		}
		_, err := command("xcrun", "simctl", "launch", "booted", config.Application.Identifier)
		return err
	case "ios-device":
		if options.Device == "" {
			return fmt.Errorf("ios-device run requires -device with a devicectl device identifier")
		}
		if config.Targets.IOS.SigningIdentity == "" {
			return fmt.Errorf("ios-device run requires targets.ios.signing_identity")
		}
		if err := buildIOSDevice(config, options); err != nil {
			return err
		}
		app := filepath.Join(options.Dist, config.Application.Name+"-device.app")
		if _, err := command("xcrun", "devicectl", "device", "install", "app", "--device", options.Device, app); err != nil {
			return err
		}
		_, err := command("xcrun", "devicectl", "device", "process", "launch", "--device", options.Device, config.Application.Identifier)
		return err
	case "android", "android-emulator", "android-device":
		return androidUnavailable(config)
	default:
		return fmt.Errorf("target %q cannot be run on this host", target)
	}
}

func buildWeb(config Config, options Options) error {
	if config.Commands.Web == "" {
		return fmt.Errorf("commands.web is not configured")
	}
	out := filepath.Join(options.Root, config.Targets.Web.Output)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	env := map[string]string{"GOOS": "js", "GOARCH": "wasm"}
	if err := goBuild(config.Commands.Web, filepath.Join(out, strings.ToLower(config.Application.Name)+".wasm"), env); err != nil {
		return err
	}
	goroot, err := command("go", "env", "GOROOT")
	if err != nil {
		return err
	}
	if err := copyFile(filepath.Join(out, "wasm_exec.js"), filepath.Join(strings.TrimSpace(goroot), "lib", "wasm", "wasm_exec.js")); err != nil {
		return err
	}
	if config.Targets.Web.Loader != "" {
		if err := copyFile(filepath.Join(out, "cadence-loader.js"), filepath.Join(options.Root, config.Targets.Web.Loader)); err != nil {
			return err
		}
	}
	return nil
}

func buildDesktopCross(config Config, target string, options Options) error {
	parts := strings.Split(target, "-")
	if len(parts) != 3 || parts[0] != "desktop" {
		return fmt.Errorf("unknown target %q", target)
	}
	goos := parts[1]
	goarch := parts[2]
	switch goos {
	case "windows", "darwin", "linux":
	default:
		return fmt.Errorf("unsupported desktop operating system %q", goos)
	}
	switch goarch {
	case "amd64", "arm64":
	default:
		return fmt.Errorf("unsupported desktop architecture %q", goarch)
	}
	suffix := ""
	if goos == "windows" {
		suffix = ".exe"
	}
	name := fmt.Sprintf("%s-%s-%s%s", strings.ToLower(config.Application.Name), goos, goarch, suffix)
	env := map[string]string{"GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": "1"}
	if goos != runtime.GOOS || goarch != runtime.GOARCH {
		key := goos + "-" + goarch
		toolchain, ok := config.Targets.Desktop.Toolchains[key]
		if !ok || toolchain.CC == "" {
			return fmt.Errorf("desktop target %s requires targets.desktop.toolchains.%s.cc", target, key)
		}
		if _, err := exec.LookPath(toolchain.CC); err != nil {
			return fmt.Errorf("desktop target %s compiler %q is unavailable", target, toolchain.CC)
		}
		env["CC"] = toolchain.CC
		if toolchain.CXX != "" {
			env["CXX"] = toolchain.CXX
		}
	}
	if err := goBuild(config.Commands.Desktop, filepath.Join(options.Dist, name), env); err != nil {
		return fmt.Errorf("desktop target %s requires a compatible Gio C toolchain: %w", target, err)
	}
	return nil
}

func buildIOSSimulator(config Config, options Options) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("ios-simulator requires macOS and Xcode")
	}
	if config.Commands.Mobile == "" {
		return fmt.Errorf("commands.mobile is not configured")
	}
	if _, err := command("xcrun", "--sdk", "iphonesimulator", "--show-sdk-path"); err != nil {
		return fmt.Errorf("iOS Simulator SDK is unavailable: %w", err)
	}
	sdk, _ := command("xcrun", "--sdk", "iphonesimulator", "--show-sdk-path")
	clang, err := command("xcrun", "--sdk", "iphonesimulator", "--find", "clang")
	if err != nil {
		return err
	}
	arch := runtime.GOARCH
	if arch != "arm64" && arch != "amd64" {
		return fmt.Errorf("unsupported iOS simulator host architecture %q", arch)
	}
	appleArch := arch
	if arch == "amd64" {
		appleArch = "x86_64"
	}
	minimum := config.Targets.IOS.MinimumVersion
	cflags := strings.Join([]string{
		"-arch", appleArch,
		"-isysroot", strings.TrimSpace(sdk),
		"-mios-simulator-version-min=" + minimum,
		"-fobjc-arc",
	}, " ")
	app := filepath.Join(options.Dist, config.Application.Name+".app")
	if err := os.RemoveAll(app); err != nil {
		return err
	}
	if err := os.MkdirAll(app, 0o755); err != nil {
		return err
	}
	executable := filepath.Join(app, config.Application.Name)
	env := map[string]string{
		"GOOS": "ios", "GOARCH": arch, "CGO_ENABLED": "1",
		"CC": strings.TrimSpace(clang), "CXX": strings.TrimSpace(clang) + "++",
		"CGO_CFLAGS": cflags, "CGO_CXXFLAGS": cflags,
		"CGO_LDFLAGS": "-lresolv " + cflags,
	}
	if err := goBuild(config.Commands.Mobile, executable, env); err != nil {
		return err
	}
	plist := iosSimulatorPlist(config)
	if err := os.WriteFile(filepath.Join(app, "Info.plist"), []byte(plist), 0o644); err != nil {
		return err
	}
	if _, err := command("plutil", "-convert", "binary1", filepath.Join(app, "Info.plist")); err != nil {
		return err
	}
	return nil
}

func buildIOSDevice(config Config, options Options) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("ios-device requires macOS and Xcode")
	}
	if config.Commands.Mobile == "" {
		return fmt.Errorf("commands.mobile is not configured")
	}
	sdk, err := command("xcrun", "--sdk", "iphoneos", "--show-sdk-path")
	if err != nil {
		return fmt.Errorf("iOS device SDK is unavailable: %w", err)
	}
	clang, err := command("xcrun", "--sdk", "iphoneos", "--find", "clang")
	if err != nil {
		return err
	}
	minimum := config.Targets.IOS.MinimumVersion
	cflags := strings.Join([]string{
		"-arch", "arm64",
		"-isysroot", strings.TrimSpace(sdk),
		"-miphoneos-version-min=" + minimum,
		"-fobjc-arc",
	}, " ")
	app := filepath.Join(options.Dist, config.Application.Name+"-device.app")
	if err := os.RemoveAll(app); err != nil {
		return err
	}
	if err := os.MkdirAll(app, 0o755); err != nil {
		return err
	}
	executable := filepath.Join(app, config.Application.Name)
	env := map[string]string{
		"GOOS": "ios", "GOARCH": "arm64", "CGO_ENABLED": "1",
		"CC": strings.TrimSpace(clang), "CXX": strings.TrimSpace(clang) + "++",
		"CGO_CFLAGS": cflags, "CGO_CXXFLAGS": cflags,
		"CGO_LDFLAGS": "-lresolv " + cflags,
	}
	if err := goBuild(config.Commands.Mobile, executable, env); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(app, "Info.plist"), []byte(iosDevicePlist(config)), 0o644); err != nil {
		return err
	}
	if _, err := command("plutil", "-convert", "binary1", filepath.Join(app, "Info.plist")); err != nil {
		return err
	}
	if config.Targets.IOS.ProvisioningProfile != "" {
		if err := copyFile(filepath.Join(app, "embedded.mobileprovision"), config.Targets.IOS.ProvisioningProfile); err != nil {
			return err
		}
	}
	if config.Targets.IOS.SigningIdentity != "" {
		if _, err := command("codesign", "--force", "--deep", "--sign", config.Targets.IOS.SigningIdentity, app); err != nil {
			return err
		}
	}
	return nil
}

func iosSimulatorPlist(config Config) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleDevelopmentRegion</key><string>en</string>
<key>CFBundleExecutable</key><string>%s</string>
<key>CFBundleIdentifier</key><string>%s</string>
<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
<key>CFBundleName</key><string>%s</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>%s</string>
<key>CFBundleVersion</key><string>%d</string>
<key>MinimumOSVersion</key><string>%s</string>
<key>CFBundleSupportedPlatforms</key><array><string>iPhoneSimulator</string></array>
<key>UIDeviceFamily</key><array><integer>1</integer><integer>2</integer></array>
<key>UILaunchScreen</key><true/>
<key>UISupportedInterfaceOrientations</key><array>
<string>UIInterfaceOrientationPortrait</string>
<string>UIInterfaceOrientationLandscapeLeft</string>
<string>UIInterfaceOrientationLandscapeRight</string>
</array>
</dict></plist>`,
		config.Application.Name,
		config.Application.Identifier,
		config.Application.Name,
		config.Application.Version,
		config.Application.Build,
		config.Targets.IOS.MinimumVersion,
	)
}

func iosDevicePlist(config Config) string {
	return strings.ReplaceAll(
		iosSimulatorPlist(config),
		"<string>iPhoneSimulator</string>",
		"<string>iPhoneOS</string>",
	)
}

func doctor(config Config, target string) error {
	switch target {
	case "all":
		targets := []string{"web", "desktop", "tui", "ios", "android"}
		var failures []string
		for _, candidate := range targets {
			if err := doctor(config, candidate); err != nil {
				failures = append(failures, candidate+": "+err.Error())
			}
		}
		if len(failures) > 0 {
			return fmt.Errorf("unavailable targets:\n  %s", strings.Join(failures, "\n  "))
		}
		return nil
	case "web", "desktop", "tui":
		_, err := command("go", "version")
		return err
	case "ios", "ios-simulator":
		if runtime.GOOS != "darwin" {
			return fmt.Errorf("requires macOS")
		}
		_, err := command("xcrun", "--sdk", "iphonesimulator", "--show-sdk-path")
		return err
	case "ios-device":
		if runtime.GOOS != "darwin" {
			return fmt.Errorf("requires macOS")
		}
		_, err := command("xcrun", "--sdk", "iphoneos", "--show-sdk-path")
		return err
	case "android", "android-emulator", "android-device":
		return androidUnavailable(config)
	default:
		return fmt.Errorf("unknown doctor target %q", target)
	}
}

func androidUnavailable(config Config) error {
	missing := []string{}
	if os.Getenv("ANDROID_HOME") == "" && os.Getenv("ANDROID_SDK_ROOT") == "" {
		missing = append(missing, "ANDROID_HOME or ANDROID_SDK_ROOT")
	}
	for _, tool := range []string{"adb", "sdkmanager"} {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}
	if len(missing) == 0 {
		return fmt.Errorf("Android SDK found; native Quicken APK packaging is not enabled yet")
	}
	return fmt.Errorf(
		"Android target is configured (min SDK %d, target SDK %d) but unavailable; missing: %s",
		config.Targets.Android.MinimumSDK,
		config.Targets.Android.TargetSDK,
		strings.Join(missing, ", "),
	)
}

func artifactName(config Config, target string) string {
	name := strings.ToLower(config.Application.Name) + "-" + target
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func goBuild(pkg, output string, extra map[string]string) error {
	if pkg == "" {
		return fmt.Errorf("target command package is not configured")
	}
	cmd := exec.Command("go", "build", "-o", output, pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = mergedEnv(extra)
	return cmd.Run()
}

func foreground(pkg string, extra map[string]string) error {
	if pkg == "" {
		return fmt.Errorf("target command package is not configured")
	}
	cmd := exec.Command("go", "run", pkg)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = mergedEnv(extra)
	return cmd.Run()
}

func mergedEnv(extra map[string]string) []string {
	env := os.Environ()
	for key, value := range extra {
		env = append(env, key+"="+value)
	}
	return env
}

func command(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func copyFile(destination, source string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o644)
}

func BuildNumber(config Config) string {
	return config.Application.Version + "." + strconv.Itoa(config.Application.Build)
}
