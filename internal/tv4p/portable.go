package tv4p

// PortableConfig is a "clean export" format similar to generator output:
// - no internal IDs/types
// - no tv4p raw fields
// Intended for copying road tool configuration between projects.
type PortableConfig struct {
	Types          []PortableRoadType      `json:"road_types"`                // road types
	CrossroadTypes []PortableCrossroadType `json:"crossroad_types,omitempty"` // crossroad types
}

// PortableRoadType is a road type in the portable config.
type PortableRoadType struct {
	Name           string             `json:"name"`                // road type name (e.g. asf1)
	StraightParts  []PortableRoadPart `json:"starting_parts"`      // starting parts
	CornerParts    []PortableRoadPart `json:"corner_parts"`        // curved corner parts
	TerminatorPart []PortableRoadPart `json:"terminator_parts"`    // road ending parts
	KeyColor       Color              `json:"key_parts_color"`     // primary color for key parts
	NormalColor    Color              `json:"normal_parts_color"`  // normal parts color
	KeyCustom      bool               `json:"key_parts_custom"`    // key parts uses custom color
	NormalCustom   bool               `json:"normal_parts_custom"` // normal parts uses custom color
}

// PortableRoadPart is a road part in the portable config.
type PortableRoadPart struct {
	Name string `json:"name"`        // part name (e.g. asf2_7 100)
	Path string `json:"object_file"` // Object File path from UI (p3d)
}

// PortableCrossroadType is a crossroad type in the portable config.
type PortableCrossroadType struct {
	Connections CrossroadConnections `json:"connections,omitempty"`  // A/B/C/D road type names
	Name        string               `json:"name"`                   // crossroad type name (e.g. kr_t_asf1_asf2)
	Model       string               `json:"model"`                  // model path (e.g. P:\DZ\structures\roads\Parts\kr_t_asf1_asf2.p3d)
	Default     string               `json:"default,omitempty"`      // default crossroad type for a road type name
	Color       Color                `json:"color"`                  // color (e.g. 0x000000FF)
	ColorCustom bool                 `json:"color_custom,omitempty"` // if false, TB uses standard color sentinel
}

// ToPortableConfig converts a RoadConfig to a PortableConfig.
func ToPortableConfig(cfg RoadConfig) PortableConfig {
	out := PortableConfig{}

	for _, rt := range cfg.Types {
		prt := PortableRoadType{
			Name:         rt.Name,
			KeyColor:     rt.KeyColor,
			NormalColor:  rt.NormalColor,
			KeyCustom:    rt.KeyCustom,
			NormalCustom: rt.NormalCustom,
		}

		for _, p := range rt.StraightParts {
			prt.StraightParts = append(prt.StraightParts, PortableRoadPart{Name: p.Name, Path: p.Path})
		}
		for _, p := range rt.CornerParts {
			prt.CornerParts = append(prt.CornerParts, PortableRoadPart{Name: p.Name, Path: p.Path})
		}
		for _, p := range rt.TerminatorPart {
			prt.TerminatorPart = append(prt.TerminatorPart, PortableRoadPart{Name: p.Name, Path: p.Path})
		}

		out.Types = append(out.Types, prt)
	}

	for _, cr := range cfg.CrossroadTypes {
		out.CrossroadTypes = append(out.CrossroadTypes, PortableCrossroadType{
			Name:        cr.Name,
			Model:       cr.Model,
			Color:       cr.Color,
			ColorCustom: cr.ColorCustom,
			Connections: cr.Connections,
			Default:     cr.Default,
		})
	}

	return out
}
