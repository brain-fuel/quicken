//go:build !js || !wasm

package dom

import (
	"goforge.dev/cadence/program"
	browser "goforge.dev/quicken/web/browser"
)

type RemoteExecutor[Msg any] struct {
	Manifest browser.Manifest
	Fallback program.Executor[Msg]
}

func NewRemoteExecutor[Msg any](
	manifest browser.Manifest,
	fallback program.Executor[Msg],
) *RemoteExecutor[Msg] {
	return &RemoteExecutor[Msg]{Manifest: manifest, Fallback: fallback}
}

func (e *RemoteExecutor[Msg]) Execute(cmd program.Cmd[Msg], dispatch program.Dispatch[Msg]) {
	if e.Fallback != nil {
		e.Fallback.Execute(cmd, dispatch)
	}
}
