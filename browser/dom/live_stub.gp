//go:build !js || !wasm

package dom

import (
	"goforge.dev/cadence/program"
	browser "goforge.dev/quicken/web/browser"
)

type LiveClient[Model any, Msg any] struct{}

func MountLive[Model any, Msg any](
	options Options[Model, Msg],
	modelCodec program.Codec[Model],
	messageCodec program.Codec[Msg],
) (*Runtime[Model, Msg], *LiveClient[Model, Msg], error) {
	return nil, nil, ErrUnavailable
}

func NewLiveClient[Model any, Msg any](
	manifest browser.Manifest,
	runtime *Runtime[Model, Msg],
	modelCodec program.Codec[Model],
	messageCodec program.Codec[Msg],
) (*LiveClient[Model, Msg], error) {
	return nil, ErrUnavailable
}

func (c *LiveClient[Model, Msg]) Connect() error { return ErrUnavailable }
func (c *LiveClient[Model, Msg]) Send(Msg)        {}
func (c *LiveClient[Model, Msg]) Close()          {}
