package mobile

import "goforge.dev/cadence/program"

const (
	EffectPermission = "quicken.mobile.permission.v1"
	EffectShare       = "quicken.mobile.share.v1"
	EffectHaptic      = "quicken.mobile.haptic.v1"
	EffectLocation    = "quicken.mobile.location.v1"
)

type PermissionRequest struct {
	Permission Permission
}

type ShareRequest struct {
	Title string
	Text  string
	URL   string
}

type HapticRequest struct {
	Kind string
}

func RequestPermission[Msg any](permission Permission) program.Cmd[Msg] {
	return program.Effect[Msg](
		"mobile.permission",
		program.CapabilityGUI(),
		EffectPermission,
		PermissionRequest{Permission: permission},
	)
}

func Share[Msg any](request ShareRequest) program.Cmd[Msg] {
	return program.Effect[Msg](
		"mobile.share",
		program.CapabilityGUI(),
		EffectShare,
		request,
	)
}

func Haptic[Msg any](kind string) program.Cmd[Msg] {
	return program.Effect[Msg](
		"mobile.haptic",
		program.CapabilityGUI(),
		EffectHaptic,
		HapticRequest{Kind: kind},
	)
}
