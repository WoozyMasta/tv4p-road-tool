package main

import (
	"os"

	"github.com/woozymasta/tv4p-road-tool/internal/tv4p"
)

type patchCmd struct {
	Args struct {
		Input  string `positional-arg-name:"IN" required:"true" description:"Input tv4p file"`
		Config string `positional-arg-name:"CONFIG" required:"true" description:"Config file (yaml/json)"`
		Output string `positional-arg-name:"OUT" description:"Output tv4p file (default: overwrite input)"`
	} `positional-args:"true"`

	Append bool `short:"a" long:"append" description:"Append to existing road types instead of overwriting"`
}

// Execute patches the road types config into the input tv4p file.
func (c *patchCmd) Execute(_ []string) error {
	cfg, err := readConfig(c.Args.Config)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(c.Args.Input)
	if err != nil {
		return err
	}

	if c.Append && len(cfg.Types) > 0 {
		cfg, err = mergeConfigWithFile(cfg, data)
		if err != nil {
			return err
		}
	}

	out, err := tv4p.PatchRoadTypes(data, cfg)
	if err != nil {
		return err
	}

	outPath := c.Args.Output
	if outPath == "" {
		outPath = c.Args.Input
	}

	if err := os.WriteFile(outPath, out, 0o600); err != nil {
		return err
	}

	printPatchStats(cfg, outPath)

	return nil
}
