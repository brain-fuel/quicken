package main

import (
	"log"

	"goforge.dev/cadence/program"
	"goforge.dev/quicken/web/browser/dom"
	taskboard "goforge.dev/cadence-taskboard"
)

func main() {
	manifests, err := dom.ReadManifests()
	if err != nil {
		log.Print(err)
		return
	}
	for _, manifest := range manifests {
		current := manifest
		options := dom.Options[taskboard.Model, taskboard.Msg]{
			Manifest: current,
			Logic: taskboard.Logic(taskboard.InitialModel(), taskboard.RemoteSave),
			View: taskboard.BrowserView,
			Executor: dom.NewRemoteExecutor[taskboard.Msg](
				current,
				program.ExecutorFunc[taskboard.Msg](
					func(program.Cmd[taskboard.Msg], program.Dispatch[taskboard.Msg]) {},
				),
			),
		}
		if current.ProgramID == taskboard.LiveID {
			_, _, err = dom.MountLive(
				options,
				taskboard.ModelCodec(),
				taskboard.MessageCodec(),
			)
		} else {
			_, err = dom.Mount(options)
		}
		if err != nil && err != dom.ErrUnavailable {
			log.Print(err)
		}
	}
	select {}
}
