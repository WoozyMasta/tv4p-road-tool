package tv4p

import (
	"fmt"
	"strings"
)

// ValidateCrossroads validates crossroad config against the provided road types list.
// This is intended to run before patching to catch bad defaults or typos early.
func ValidateCrossroads(crossroads []CrossroadType, roadTypes []RoadType) error {
	nameSet := map[string]struct{}{}
	for _, rt := range roadTypes {
		if strings.TrimSpace(rt.Name) == "" {
			continue
		}
		nameSet[strings.ToLower(rt.Name)] = struct{}{}
	}

	seenDefault := map[string]string{} // roadTypeLower -> crossroadName

	for _, cr := range crossroads {
		// Validate connection names.
		for side, v := range map[string]string{
			"A": cr.Connections.A,
			"B": cr.Connections.B,
			"C": cr.Connections.C,
			"D": cr.Connections.D,
		} {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if _, ok := nameSet[strings.ToLower(v)]; !ok {
				return fmt.Errorf("crossroad %q: unknown road type for %s: %q", cr.Name, side, v)
			}
		}

		// Validate default mapping.
		if strings.TrimSpace(cr.Default) != "" {
			d := strings.ToLower(strings.TrimSpace(cr.Default))
			if _, ok := nameSet[d]; !ok {
				return fmt.Errorf("crossroad %q: default refers to unknown road type %q", cr.Name, cr.Default)
			}
			if !crossroadHasRoadType(cr, cr.Default) {
				return fmt.Errorf("crossroad %q: default=%q but this road type is not present in connections", cr.Name, cr.Default)
			}
			if prev, exists := seenDefault[d]; exists {
				return fmt.Errorf("duplicate crossroad default for %q: %q and %q", cr.Default, prev, cr.Name)
			}
			seenDefault[d] = cr.Name
		}
	}

	return nil
}
