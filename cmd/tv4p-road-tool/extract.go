package main

import (
	"os"
	"strings"

	"github.com/woozymasta/tv4p-road-tool/internal/tv4p"
)

type extractCmd struct {
	Args struct {
		Input  string `positional-arg-name:"IN" required:"true" description:"Input tv4p file"`
		Output string `positional-arg-name:"OUT" description:"Output config file (default: stdout)"`
	} `positional-args:"true"`

	Format string `short:"f" long:"format" choice:"yaml" choice:"json" default:"yaml" description:"Output format"`
}

// Execute extracts the road types config from the input tv4p file.
func (c *extractCmd) Execute(_ []string) error {
	format := strings.ToLower(c.Format)
	if format == "" {
		format = "yaml"
	}

	data, err := os.ReadFile(c.Args.Input)
	if err != nil {
		return err
	}

	block, err := tv4p.ParseRoadTypes(data)
	if err != nil {
		return err
	}

	cfg := tv4p.RoadConfig{Types: block.Types}
	out, err := encodeConfig(cfg, format)
	if err != nil {
		return err
	}

	if c.Args.Output == "" {
		_, err = os.Stdout.Write(out)
		return err
	}

	return os.WriteFile(c.Args.Output, out, 0o600)
}
