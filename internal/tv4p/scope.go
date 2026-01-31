package tv4p

// Scope controls which parts of the Road Tool configuration are processed.
type Scope string

const (
	// ScopeAll means patch/extract/generate everything (roads + crossroads).
	ScopeAll Scope = "all"
	// ScopeRoads means patch/extract/generate only road types.
	ScopeRoads Scope = "roads"
	// ScopeCrossroad means patch/extract/generate only crossroads.
	ScopeCrossroad Scope = "crossroads"
)

// IncludesRoads returns true if the scope includes road types.
func (s Scope) IncludesRoads() bool {
	return s == ScopeAll || s == ScopeRoads
}

// IncludesCrossroads returns true if the scope includes crossroad types.
func (s Scope) IncludesCrossroads() bool {
	return s == ScopeAll || s == ScopeCrossroad
}
