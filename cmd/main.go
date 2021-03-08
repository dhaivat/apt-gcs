package main

import (
	. "github.com/dhaivat/apt-gcs"
)

func main() {
	InitConfig()
	a := AptMethod{}
	a.SendCapabilities()
	a.Run()
}
