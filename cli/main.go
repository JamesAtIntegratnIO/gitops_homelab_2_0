package main

import (
	"os"

	"github.com/jamesatintegratnio/hctl/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
