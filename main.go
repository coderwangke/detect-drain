package main

import (
	"fmt"
	"github.com/coderwangke/detect-drain/cmd"
	"os"
)

func main() {
	command := cmd.NewDetectDrainCmd()
	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
