package main

import (
	"runtime"

	"mountain-mogul/internal/settings"
)

func init() {
	runtime.LockOSThread()
	settings.Init()
}
