package main

import (
	"bufio"
	"os"

	. "github.com/dhaivat/apt-gcs"
)

func main() {
	InitConfig()
	a := AptMethod{
		bufio.NewReader(os.Stdin),
	}
	a.SendCapabilities()
	a.Run()
}
