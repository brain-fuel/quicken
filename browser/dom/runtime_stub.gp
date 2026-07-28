//go:build !js || !wasm

package dom

import (
	"errors"

	browser "goforge.dev/quicken/web/browser"
)

var ErrUnavailable = errors.New("quicken/web/browser/dom: requires GOOS=js GOARCH=wasm")

func ReadManifests() ([]browser.Manifest, error) {
	return nil, ErrUnavailable
}

func Mount[Model any, Msg any](Options[Model, Msg]) (*Runtime[Model, Msg], error) {
	return nil, ErrUnavailable
}
