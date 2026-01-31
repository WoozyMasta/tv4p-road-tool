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
	Scope    string `long:"scope" choice:"all" choice:"roads" choice:"crossroads" default:"all" description:"What to generate: roads, crossroads, or all"`

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

	scope := tv4p.Scope(c.Scope)
	outCfg := filterConfigByScope(cfg, scope)
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

// generateConfig generates the road types config from the disk.
func generateConfig(paths []string, gameRoot string, noOgol bool, verbose bool) (tv4p.RoadConfig, error) {
	types := map[string]*tv4p.RoadType{}
	crossroads := map[string]*tv4p.CrossroadType{}
	root := cleanAbs(gameRoot)

	var (
		totalFiles, filesP3D, filesMLOD, filesODOL            int
		filesNameReject, filesKindReject, filesCrossroadAdded int
		filesAdded                                            int
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
				crName, ok := roadparts.ParseCrossroadBase(parsed.Name)
				if !ok {
					filesKindReject++
					if verbose {
						fmt.Fprintf(os.Stderr, "skip: %s (crossroad name reject)\n", path)
					}
					return nil
				}

				name := parsed.Name
				if _, exists := crossroads[name]; !exists {
					modelPath := toCrossroadModelPath(path, root)
					cr := &tv4p.CrossroadType{
						Name:  name,
						Model: modelPath,
						// Use computed, custom color (can be overridden in YAML if needed).
						ColorCustom: true,
						Color:       tv4p.Color{R: 255, G: 0, B: 255, A: 255},
						Connections: tv4p.CrossroadConnections{
							A: crName.AB,
							B: crName.AB,
							C: crName.C,
							D: crName.D,
						},
					}
					// For T-shape, D is not used.
					if crName.Shape == roadparts.CrossroadShapeT {
						cr.Connections.D = ""
					}

					crossroads[name] = cr
				}

				filesCrossroadAdded++
				if verbose {
					fmt.Fprintf(os.Stderr, "add: %s (crossroad)\n", path)
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

	// Now that we have the final road types list (and therefore palette decisions),
	// compute crossroad colors from their A/B/C(/D) connections.
	roadTypeNames := map[string]struct{}{}
	for _, rt := range list {
		roadTypeNames[rt.Name] = struct{}{}
	}
	for _, cr := range crossroads {
		colors := crossroadConnectionColors(cr.Connections, roadTypeNames)
		if len(colors) == 0 {
			// Fallback UI color if nothing is resolvable.
			cr.Color = tv4p.Color{R: 255, G: 0, B: 255, A: 255}
			continue
		}

		mixed := roadparts.MixColors(colors...)
		// Shift to a darker shade vs the underlying road colors.
		cr.Color = roadparts.DarkenColor(mixed, 0.75)
	}

	var crossList []tv4p.CrossroadType
	for _, cr := range crossroads {
		crossList = append(crossList, *cr)
	}
	sort.Slice(crossList, func(i, j int) bool { return crossList[i].Name < crossList[j].Name })

	// Mark defaults explicitly (can be edited in YAML later).
	assignCrossroadDefaults(list, crossList)

	if verbose {
		fmt.Fprintf(os.Stderr, "summary: files=%d p3d=%d mlod=%d odol=%d name_reject=%d kind_reject=%d crossroad=%d added=%d types=%d\n",
			totalFiles, filesP3D, filesMLOD, filesODOL, filesNameReject, filesKindReject, filesCrossroadAdded, filesAdded, len(list))
	}

	if filesP3D > 0 && filesMLOD == 0 {
		fmt.Fprint(os.Stderr, `WARNING: no MLOD road models found.
Terrain Builder needs MLOD models to read sizes/metadata for Road Tool.

Download MLOD roads from: https://github.com/BohemiaInteractive/DayZ-Misc

Then copy the road models into your game root (e.g. DZ/structures/roads/Parts).
By default the game ships ODOL (binarized) models, which are not suitable here.
`)
	}

	return tv4p.RoadConfig{Types: list, CrossroadTypes: crossList}, nil
}

// assignCrossroadDefaults assigns the default crossroad for each road type.
func assignCrossroadDefaults(roadTypes []tv4p.RoadType, crossroads []tv4p.CrossroadType) {
	// Ensure there is at most one default per road type.
	// If a crossroad already has Default set (rare in generator), keep it.
	seen := map[string]struct{}{}
	for _, cr := range crossroads {
		if strings.TrimSpace(cr.Default) == "" {
			continue
		}
		seen[strings.ToLower(strings.TrimSpace(cr.Default))] = struct{}{}
	}

	score := func(cr tv4p.CrossroadType, want string) int {
		want = strings.ToLower(want)
		abA := strings.ToLower(cr.Connections.A)
		abB := strings.ToLower(cr.Connections.B)
		c := strings.ToLower(cr.Connections.C)
		d := strings.ToLower(cr.Connections.D)

		shape := 0
		if strings.HasPrefix(cr.Name, "kr_t_") {
			shape = 2
		} else if strings.HasPrefix(cr.Name, "kr_x_") {
			shape = 1
		}

		if abA == want && abB == want {
			return 100 + shape
		}
		if abA == want || abB == want {
			return 80 + shape
		}
		if c == want || d == want {
			return 60 + shape
		}
		return -1
	}

	for _, rt := range roadTypes {
		want := strings.TrimSpace(rt.Name)
		if want == "" {
			continue
		}
		key := strings.ToLower(want)
		if _, ok := seen[key]; ok {
			continue
		}

		best := -1
		bestScore := -1
		for i := range crossroads {
			if strings.TrimSpace(crossroads[i].Default) != "" {
				continue
			}
			s := score(crossroads[i], want)
			if s > bestScore {
				bestScore = s
				best = i
			}
		}
		if best >= 0 && bestScore >= 0 {
			crossroads[best].Default = want
			seen[key] = struct{}{}
		}
	}
}

// crossroadConnectionColors computes the colors for a crossroad based on its connections.
func crossroadConnectionColors(c tv4p.CrossroadConnections, known map[string]struct{}) []tv4p.Color {
	var out []tv4p.Color

	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := known[name]; !ok {
			// Still allow palette hashing fallback, but keep behavior stable:
			// do not include unknown types in the mix.
			return
		}
		normal, _, ok := roadparts.Palette(name)
		if !ok {
			return
		}
		out = append(out, normal)
	}

	// Weighting is via duplicates, so A and B contribute twice if same type.
	add(c.A)
	add(c.B)
	add(c.C)
	add(c.D)

	return out
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

// toCrossroadModelPath converts a crossroad model path to the expected Road Tool format.
// For crossroads Terrain Builder stores an absolute P:\ style path in observed files.
func toCrossroadModelPath(path string, gameRoot string) string {
	abs := cleanAbs(path)
	if gameRoot != "" {
		rel, err := filepath.Rel(gameRoot, abs)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return toBackslashes(filepath.Join(gameRoot, rel))
		}
	}

	return toBackslashes(abs)
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
