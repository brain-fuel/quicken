package quicken

import "goforge.dev/cadence"

// The region model and content substrate live in cadence, the foundation
// module. quicken re-exports them under its own names so its transports,
// pages, and authoring adapters (and their tests) keep working unchanged while
// depending on the shared foundation.
type (
	Tree          = cadence.Tree
	Region        = cadence.Region
	LiveRegion    = cadence.LiveRegion
	RenderContext = cadence.RenderContext
	State         = cadence.State
	Params        = cadence.Params
	Payload       = cadence.Payload
)

var (
	Text       = cadence.Text
	Slots      = cadence.Slots
	RegionFunc = cadence.RegionFunc
)
