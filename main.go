package main

import (
	"os"

	"github.com/konono/aw/internal/cmd"
)

func main() {
	os.Exit(cmd.Run(os.Args[1:]))
}
