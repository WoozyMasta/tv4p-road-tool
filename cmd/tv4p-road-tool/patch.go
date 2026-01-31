package main

import (
	"os"
	"strings"

	"github.com/woozymasta/tv4p-road-tool/internal/tv4p"
)

type patchCmd struct {
	Args struct {
		Input  string `positional-arg-name:"IN" required:"true" description:"Input tv4p file"`
		Config string `positional-arg-name:"CONFIG" required:"true" description:"Config file (yaml/json)"`
		Output string `positional-arg-name:"OUT" description:"Output tv4p file (default: overwrite input)"`
	} `positional-args:"true"`

	Scope         string `short:"s" long:"scope" choice:"all" choice:"roads" choice:"crossroads" default:"all" description:"What to patch: roads, crossroads, or all"`
	Append        bool   `short:"a" long:"append" description:"Append to existing road types instead of overwriting"`
	AllCrossroads bool   `long:"all-crossroads" description:"Write all crossroad definitions. Default behavior is to write only defaults (one per road type) to avoid Terrain Builder crossroad variant issues."`
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

	scope := tv4p.Scope(c.Scope)

	// For crossroads-only configs it is convenient to omit road_types in YAML.
	// We still need road types for validation and for human-friendly stats output.
	if scope.IncludesCrossroads() && len(cfg.Types) == 0 {
		existing, err := tv4p.ParseRoadTypes(data)
		if err != nil {
			return err
		}
		cfg.Types = existing.Types
	}

	// By default, only write one (default) crossroad per road type.
	// Terrain Builder often ignores crossroad variant selection and behaves as if it uses 0x89[roadTypeIndex].
	if scope.IncludesCrossroads() && cfg.CrossroadTypes != nil && !c.AllCrossroads {
		cfg.CrossroadTypes = selectDefaultCrossroads(cfg.CrossroadTypes, cfg.Types)
	}

	if c.Append && scope.IncludesRoads() && len(cfg.Types) > 0 {
		cfg, err = mergeConfigWithFile(cfg, data)
		if err != nil {
			return err
		}
	}

	out, err := tv4p.PatchRoadTool(data, cfg, scope)
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

// selectDefaultCrossroads selects the default crossroad for each road type.
func selectDefaultCrossroads(all []tv4p.CrossroadType, roadTypes []tv4p.RoadType) []tv4p.CrossroadType {
	if len(all) == 0 || len(roadTypes) == 0 {
		return all
	}

	// Build explicit defaults map: roadTypeLower -> index in all.
	explicit := map[string]int{}
	for i := range all {
		d := strings.TrimSpace(all[i].Default)
		if d == "" {
			continue
		}
		explicit[strings.ToLower(d)] = i
	}

	shapeScore := func(cr tv4p.CrossroadType) int {
		if strings.HasPrefix(cr.Name, "kr_t_") {
			return 2
		}
		if strings.HasPrefix(cr.Name, "kr_x_") {
			return 1
		}
		return 0
	}

	matchScore := func(cr tv4p.CrossroadType, want string) int {
		want = strings.ToLower(want)
		if strings.TrimSpace(cr.Default) != "" && strings.ToLower(strings.TrimSpace(cr.Default)) == want {
			return 1000 + shapeScore(cr)
		}

		abA := strings.ToLower(cr.Connections.A)
		abB := strings.ToLower(cr.Connections.B)
		c := strings.ToLower(cr.Connections.C)
		d := strings.ToLower(cr.Connections.D)

		if abA == want && abB == want {
			return 100 + shapeScore(cr)
		}
		if abA == want || abB == want {
			return 80 + shapeScore(cr)
		}
		if c == want || d == want {
			return 60 + shapeScore(cr)
		}
		return -1
	}

	var out []tv4p.CrossroadType

	for _, rt := range roadTypes {
		want := strings.TrimSpace(rt.Name)
		if want == "" {
			continue
		}
		key := strings.ToLower(want)

		// Explicit default wins.
		if idx, ok := explicit[key]; ok {
			cr := all[idx]
			if strings.TrimSpace(cr.Default) == "" {
				cr.Default = want
			}
			out = append(out, cr)
			continue
		}

		// Otherwise pick the best match for this road type.
		best := -1
		bestScore := -1
		for i := range all {
			s := matchScore(all[i], want)
			if s > bestScore {
				bestScore = s
				best = i
			}
		}
		if best >= 0 && bestScore >= 0 {
			cr := all[best]
			// Mark it explicitly so it's visible/editable in YAML after extract.
			if strings.TrimSpace(cr.Default) == "" {
				cr.Default = want
			}
			out = append(out, cr)
		}
	}

	// If we couldn't pick anything, fall back to original (safer than empty list).
	if len(out) == 0 {
		return all
	}

	return out
}
