package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/woozymasta/tv4p-road-tool/internal/p3d"
	"github.com/woozymasta/tv4p-road-tool/internal/roadparts"
	"github.com/woozymasta/tv4p-road-tool/internal/tv4p"
)

type generateCmd struct {
	Format   string `short:"f" long:"format" choice:"yaml" choice:"json" default:"yaml" description:"Output format"`
	GameRoot string `short:"g" long:"game-root" description:"Game root directory"`

	Args struct {
		Output string `positional-arg-name:"OUT" description:"Output config file (default: stdout)"`
	} `positional-args:"true"`

	Paths   []string `short:"p" long:"path" default:"DZ/structures/roads/Parts" default:"DZ/structures_bliss/roads/Parts" default:"DZ/structures_sakhal/roads/parts" description:"Search path (repeatable)"`
	NoOgol  bool     `long:"no-odol-check" description:"Disable ODOL/MLOD header check"`
	Verbose bool     `short:"v" long:"verbose" description:"Verbose per-file output"`
}

// Execute generates the road types config from the disk.
func (c *generateCmd) Execute(_ []string) error {
	format := strings.ToLower(c.Format)
	if format == "" {
		format = "yaml"
	}

	paths := resolvePaths(c.GameRoot, c.Paths)
	if len(paths) == 0 {
		return errors.New("no valid search paths")
	}

	cfg, err := generateConfig(paths, c.GameRoot, c.NoOgol, c.Verbose)
	if err != nil {
		return err
	}

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

// generateConfig generates the road types config from the disk.
func generateConfig(paths []string, gameRoot string, noOgol bool, verbose bool) (tv4p.RoadConfig, error) {
	types := map[string]*tv4p.RoadType{}
	root := cleanAbs(gameRoot)

	var (
		totalFiles, filesP3D, filesMLOD, filesODOL             int
		filesNameReject, filesKindReject, filesCrossroadReject int
		filesAdded                                             int
	)

	for _, p := range paths {
		err := filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "skip: %s (walk error)\n", path)
				}
				return nil
			}

			if d.IsDir() {
				return nil
			}

			totalFiles++
			if strings.ToLower(filepath.Ext(d.Name())) != ".p3d" {
				return nil
			}

			filesP3D++
			if !noOgol {
				ok, kind, err := p3d.IsMLOD(path)
				if err != nil {
					if verbose {
						fmt.Fprintf(os.Stderr, "skip: %s (header read error)\n", path)
					}
					return nil
				}

				switch kind {
				case "MLOD":
					filesMLOD++
				case "ODOL":
					filesODOL++
				}

				if !ok {
					if verbose {
						switch kind {
						case "ODOL":
							fmt.Fprintf(os.Stderr, "skip: %s (ODOL)\n", path)
						case "UNKNOWN":
							fmt.Fprintf(os.Stderr, "skip: %s (unknown header)\n", path)
						default:
							fmt.Fprintf(os.Stderr, "skip: %s (not MLOD)\n", path)
						}
					}
					return nil
				}
			}

			parsed, ok := roadparts.ParseFile(path)
			if !ok {
				filesNameReject++
				if verbose {
					fmt.Fprintf(os.Stderr, "skip: %s (name reject)\n", path)
				}
				return nil
			}

			if parsed.Kind == roadparts.Unknown {
				filesKindReject++
				if verbose {
					fmt.Fprintf(os.Stderr, "skip: %s (kind unknown)\n", path)
				}
				return nil
			}

			if parsed.Kind == roadparts.Crossroad {
				filesCrossroadReject++
				if verbose {
					fmt.Fprintf(os.Stderr, "skip: %s (crossroad)\n", path)
				}
				return nil
			}

			rt := types[parsed.TypeName]
			if rt == nil {
				rt = &tv4p.RoadType{
					Name:         parsed.TypeName,
					Type:         0x12,
					KeyCustom:    false,
					NormalCustom: false,
				}
				applyRoadPalette(rt)
				types[parsed.TypeName] = rt
			}

			objPath := toObjectFile(path, root)
			part := tv4p.RoadPart{
				Name: parsed.Name,
				Path: objPath,
				Type: partTypeFromKind(parsed.Kind),
			}

			switch parsed.Kind {
			case roadparts.Straight:
				rt.StraightParts = append(rt.StraightParts, part)
				filesAdded++
				if verbose {
					fmt.Fprintf(os.Stderr, "add: %s (straight -> %s)\n", path, rt.Name)
				}

			case roadparts.Corner:
				rt.CornerParts = append(rt.CornerParts, part)
				filesAdded++
				if verbose {
					fmt.Fprintf(os.Stderr, "add: %s (corner -> %s)\n", path, rt.Name)
				}

			case roadparts.Terminator:
				rt.TerminatorPart = append(rt.TerminatorPart, part)
				filesAdded++
				if verbose {
					fmt.Fprintf(os.Stderr, "add: %s (terminator -> %s)\n", path, rt.Name)
				}

			case roadparts.Crosswalk:
				part.Type = 0x13
				rt.StraightParts = append(rt.StraightParts, part)
				filesAdded++
				if verbose {
					fmt.Fprintf(os.Stderr, "add: %s (crosswalk -> %s)\n", path, rt.Name)
				}
			}

			return nil
		})

		if err != nil {
			return tv4p.RoadConfig{}, err
		}
	}

	var list []tv4p.RoadType
	for _, rt := range types {
		sort.Slice(rt.StraightParts, func(i, j int) bool { return rt.StraightParts[i].Name < rt.StraightParts[j].Name })
		sort.Slice(rt.CornerParts, func(i, j int) bool { return rt.CornerParts[i].Name < rt.CornerParts[j].Name })
		sort.Slice(rt.TerminatorPart, func(i, j int) bool { return rt.TerminatorPart[i].Name < rt.TerminatorPart[j].Name })
		list = append(list, *rt)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })

	if verbose {
		fmt.Fprintf(os.Stderr, "summary: files=%d p3d=%d mlod=%d odol=%d name_reject=%d kind_reject=%d crossroad=%d added=%d types=%d\n",
			totalFiles, filesP3D, filesMLOD, filesODOL, filesNameReject, filesKindReject, filesCrossroadReject, filesAdded, len(list))
	}

	if filesP3D > 0 && filesMLOD == 0 {
		fmt.Fprint(os.Stderr, `WARNING: no MLOD road models found.
Terrain Builder needs MLOD models to read sizes/metadata for Road Tool.

Download MLOD roads from: https://github.com/BohemiaInteractive/DayZ-Misc

Then copy the road models into your game root (e.g. DZ/structures/roads/Parts).
By default the game ships ODOL (binarized) models, which are not suitable here.
`)
	}

	return tv4p.RoadConfig{Types: list}, nil
}

// partTypeFromKind converts the road part kind to the type.
func partTypeFromKind(kind roadparts.Kind) uint16 {
	switch kind {
	case roadparts.Straight:
		return 0x13
	case roadparts.Corner:
		return 0x14
	case roadparts.Terminator:
		return 0x16
	default:
		return 0
	}
}

// toObjectFile converts the path to the object file.
func toObjectFile(path string, gameRoot string) string {
	abs := cleanAbs(path)
	if gameRoot != "" {
		rel, err := filepath.Rel(gameRoot, abs)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return strings.ToLower(toBackslashes(rel))
		}
	}

	return strings.ToLower(toBackslashes(abs))
}

// applyRoadPalette applies the road palette to the road type.
func applyRoadPalette(rt *tv4p.RoadType) {
	if rt == nil {
		return
	}

	if rt.KeyCustom || rt.NormalCustom {
		return
	}

	normal, key, ok := roadparts.Palette(rt.Name)
	if !ok {
		return
	}

	rt.NormalColor = normal
	rt.KeyColor = key
	rt.KeyCustom = true
	rt.NormalCustom = true
}
