// Command tv4p-road-tool provides CLI utilities for tv4p road types.
package main

import (
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/woozymasta/tv4p-road-tool/internal/vars"
)

type rootCmd struct {
	Version  versionCmd  `command:"version" description:"Show version information"`
	Patch    patchCmd    `command:"patch" description:"Patch road types config into tv4p"`
	Extract  extractCmd  `command:"extract" description:"Extract road types config from tv4p"`
	Generate generateCmd `command:"generate" description:"Generate config from disk (not implemented)"`
}

func main() {
	var root rootCmd
	parser := flags.NewParser(&root, flags.Default)
	if _, err := parser.Parse(); err != nil {
		if fe, ok := err.(*flags.Error); ok && fe.Type == flags.ErrHelp {
			return
		}
		os.Exit(1)
	}
}

type versionCmd struct{}

// Execute prints the version information.
func (c *versionCmd) Execute(_ []string) {
	vars.Print()
	os.Exit(0)
}
