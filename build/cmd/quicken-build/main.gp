package main

import (
	"flag"
	"fmt"
	"os"

	qbuild "goforge.dev/quicken/build"
)

func main() {
	configPath := flag.String("config", "quicken.yaml", "path to quicken application configuration")
	root := flag.String("root", ".", "application root")
	dist := flag.String("dist", "", "artifact output directory")
	device := flag.String("device", "", "emulator or simulator device name")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: quicken-build [flags] <doctor|build|run> <target>")
		os.Exit(2)
	}
	config, err := qbuild.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "quicken-build:", err)
		os.Exit(1)
	}
	err = qbuild.Run(config, flag.Arg(0), flag.Arg(1), qbuild.Options{
		Root: *root, Dist: *dist, Device: *device,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "quicken-build:", err)
		os.Exit(1)
	}
}
