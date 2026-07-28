//go:build !android && !ios

package mobile

func platform() Platform { return PlatformUnknown() }
