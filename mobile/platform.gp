package mobile

type Platform enum {
	PlatformUnknown
	PlatformIOS
	PlatformAndroid
}

type Lifecycle enum {
	LifecycleForeground
	LifecycleBackground
	LifecycleInactive
}

type Permission enum {
	PermissionCamera
	PermissionLocation
	PermissionMicrophone
	PermissionNotifications
	PermissionPhotos
}

type Insets struct {
	Top, Right, Bottom, Left float32
}

type Environment struct {
	Platform  Platform
	Lifecycle Lifecycle
	SafeArea  Insets
}

func CurrentPlatform() Platform { return platform() }

// Keep enum construction in the untagged unit where the Go+ generator can
// resolve the complete variant algebra.
func iosPlatform() Platform { return PlatformIOS() }
func androidPlatform() Platform { return PlatformAndroid() }
