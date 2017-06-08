package main

import (
	. "github.com/ceocoder/apt-gcs"
)

func main() {
	InitConfig()
	a := AptMethod{}
	a.SendCapabilities()
	a.Run()
}
