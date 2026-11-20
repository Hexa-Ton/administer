package main

import (
	"os"

	"github.com/Hexa-Ton/administer/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
