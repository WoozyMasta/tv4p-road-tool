// Package roadparts parses road part filenames, assigns types and palette colors.
package roadparts

import (
	"path/filepath"
	"strings"
)

// Kind represents the type of road part.
type Kind int

const (
	// Unknown is an unrecognized part type.
	Unknown Kind = iota

	// Straight part.
	Straight

	// Corner part.
	Corner

	// Terminator part.
	Terminator

	// Crosswalk part.
	Crosswalk

	// Crossroad part.
	Crossroad
)

// Parsed represents the parsed road part information.
type Parsed struct {
	TypeName string // Type name (e.g. asf1)
	Name     string // Part name (e.g. asf1_12)
	Kind     Kind   // Part type
}

// ParseFile parses the road part information from a file path.
func ParseFile(path string) (Parsed, bool) {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	return ParseBase(base)
}

// ParseBase parses the road part information from a base name.
func ParseBase(base string) (Parsed, bool) {
	orig := base
	if strings.HasPrefix(base, "kr_t_") || strings.HasPrefix(base, "kr_x_") {
		return Parsed{TypeName: "crossroad", Name: orig, Kind: Crossroad}, true
	}

	isCrosswalk := false
	if strings.HasSuffix(base, "_crosswalk") {
		isCrosswalk = true
		base = strings.TrimSuffix(base, "_crosswalk")
	}

	isTerminator := false
	if strings.HasSuffix(base, "konec") {
		isTerminator = true
		base = strings.TrimSuffix(base, "konec")
	}

	idx := strings.LastIndex(base, "_")
	if idx <= 0 || idx == len(base)-1 {
		return Parsed{}, false
	}

	typeName := base[:idx]
	rest := base[idx+1:]
	if typeName == "" || rest == "" {
		return Parsed{}, false
	}

	if isTerminator {
		if !isDigits(rest) {
			return Parsed{}, false
		}

		return Parsed{TypeName: typeName, Name: orig, Kind: Terminator}, true
	}

	fields := strings.Fields(rest)
	if len(fields) == 2 && isDigits(fields[0]) && isDigits(fields[1]) {
		return Parsed{TypeName: typeName, Name: orig, Kind: Corner}, true
	}

	if len(fields) == 1 && isDigits(fields[0]) {
		if isCrosswalk {
			return Parsed{TypeName: typeName, Name: orig, Kind: Crosswalk}, true
		}

		return Parsed{TypeName: typeName, Name: orig, Kind: Straight}, true
	}

	return Parsed{TypeName: typeName, Name: orig, Kind: Unknown}, true
}

// isDigits checks if a string contains only digits.
func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return s != ""
}
