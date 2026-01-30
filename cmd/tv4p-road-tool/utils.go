package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/invopop/yaml"

	"github.com/woozymasta/tv4p-road-tool/internal/tv4p"
)

// readConfig reads the config from the file.
func readConfig(path string) (tv4p.RoadConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return tv4p.RoadConfig{}, err
	}

	var cfg tv4p.RoadConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return tv4p.RoadConfig{}, err
	}

	return cfg, nil
}

// encodeConfig encodes the config to the raw data.
func encodeConfig(cfg tv4p.RoadConfig, format string) ([]byte, error) {
	switch format {
	case "yaml":
		return yaml.Marshal(cfg)
	case "json":
		return json.MarshalIndent(cfg, "", "  ")
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}
}

// decodeConfig decodes the config from the raw data.
// printPatchStats prints the patch statistics.
func printPatchStats(cfg tv4p.RoadConfig, outPath string) {
	var straight, corner, terminator int
	for _, rt := range cfg.Types {
		straight += len(rt.StraightParts)
		corner += len(rt.CornerParts)
		terminator += len(rt.TerminatorPart)
	}

	fmt.Printf("patched %s\n", outPath)
	fmt.Printf("road types: %d\n", len(cfg.Types))
	fmt.Printf("starting parts: %d\n", straight)
	fmt.Printf("corner parts: %d\n", corner)
	fmt.Printf("terminator parts: %d\n", terminator)
}

// resolvePaths resolves the paths relative to the game root.
func resolvePaths(gameRoot string, paths []string) []string {
	var out []string
	root := cleanAbs(gameRoot)
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if filepath.IsAbs(p) {
			out = append(out, cleanAbs(p))
			continue
		}

		if root != "" {
			out = append(out, cleanAbs(filepath.Join(root, p)))
			continue
		}

		out = append(out, cleanAbs(p))
	}

	return out
}

// toBackslashes converts the path to backslashes.
func toBackslashes(p string) string {
	return strings.ReplaceAll(p, "/", "\\")
}

// mergeConfigWithFile merges the config with the input tv4p file.
func mergeConfigWithFile(cfg tv4p.RoadConfig, data []byte) (tv4p.RoadConfig, error) {
	existing, err := tv4p.ParseRoadTypes(data)
	if err != nil {
		return cfg, err
	}

	byName := map[string]*tv4p.RoadType{}
	for i := range existing.Types {
		rt := &existing.Types[i]
		byName[rt.Name] = rt
	}

	for _, rt := range cfg.Types {
		ex := byName[rt.Name]
		if ex == nil {
			copyRt := rt
			existing.Types = append(existing.Types, copyRt)
			continue
		}

		ex.StraightParts = append(ex.StraightParts, rt.StraightParts...)
		ex.CornerParts = append(ex.CornerParts, rt.CornerParts...)
		ex.TerminatorPart = append(ex.TerminatorPart, rt.TerminatorPart...)
		if rt.KeyCustom {
			ex.KeyCustom = true
			ex.KeyColor = rt.KeyColor
		}
		if rt.NormalCustom {
			ex.NormalCustom = true
			ex.NormalColor = rt.NormalColor
		}
		if rt.Type != 0 {
			ex.Type = rt.Type
		}
		if rt.ID != 0 {
			ex.ID = rt.ID
		}
	}

	return tv4p.RoadConfig{Types: existing.Types}, nil
}

// cleanAbs cleans a path and returns it as an absolute path.
func cleanAbs(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}

	// Windows drive root normalization
	// "P:" -> "P:\\"
	if len(p) == 2 && p[1] == ':' &&
		((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z')) {
		return strings.ToUpper(p[:1]) + `:\`
	}

	// Keep "P:\\" and "P:/" as drive root
	if len(p) == 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		return strings.ToUpper(p[:1]) + `:\`
	}

	// Cross-platform cleanup
	return filepath.Clean(p)
}
