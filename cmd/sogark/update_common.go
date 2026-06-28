package main

import "runtime"

type binaryReplaceResult struct {
	Deferred bool
}

func sogarkBinaryName() string {
	name := "sogark-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}
