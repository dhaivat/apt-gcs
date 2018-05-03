package main

import (
	. "github.com/ceocoder/apt-gcs"
)

func main() {
	a := AptMethod{}
	a.SendCapabilities()
	a.Run()
}
