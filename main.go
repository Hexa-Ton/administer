package main

import (
	"os"

	"github.com/YOUR_GITHUB_USERNAME/administer/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
