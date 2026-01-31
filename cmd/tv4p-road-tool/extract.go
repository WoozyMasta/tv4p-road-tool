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

	Format   string `short:"f" long:"format" choice:"yaml" choice:"json" default:"yaml" description:"Output format"`
	Scope    string `long:"scope" choice:"all" choice:"roads" choice:"crossroads" default:"all" description:"What to extract: roads, crossroads, or all"`
	Portable bool   `short:"p" long:"portable" description:"Export portable config: no IDs/types, no tv4p raw fields"`
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

	cfg, err := tv4p.ParseRoadToolConfig(data)
	if err != nil {
		return err
	}

	scope := tv4p.Scope(c.Scope)
	var outCfg any
	if c.Portable {
		outCfg = filterPortableByScope(tv4p.ToPortableConfig(cfg), scope)
	} else {
		outCfg = filterConfigByScope(cfg, scope)
	}

	out, err := encodeConfig(outCfg, format)
	if err != nil {
		return err
	}

	if c.Args.Output == "" {
		_, err = os.Stdout.Write(out)
		return err
	}

	return os.WriteFile(c.Args.Output, out, 0o600)
}
