package roadparts

import "github.com/woozymasta/tv4p-road-tool/internal/tv4p"

// paletteRule represents a color palette rule.
type paletteRule struct {
	Keys   []string   // Keys to match (e.g. ["snow", "runway"])
	Normal tv4p.Color // Normal color (e.g. {R: 170, G: 220, B: 255, A: 255})
	Key    tv4p.Color // Key color (e.g. {R: 90, G: 150, B: 210, A: 255})
}

// paletteRules is the list of color palette rules.
var paletteRules = []paletteRule{
	{
		Keys:   []string{"snow"},
		Normal: tv4p.Color{R: 170, G: 220, B: 255, A: 255},
		Key:    tv4p.Color{R: 90, G: 150, B: 210, A: 255},
	},
	{
		Keys:   []string{"runway"},
		Normal: tv4p.Color{R: 60, G: 120, B: 220, A: 255},
		Key:    tv4p.Color{R: 20, G: 60, B: 160, A: 255},
	},
	{
		Keys:   []string{"sidewalk"},
		Normal: tv4p.Color{R: 220, G: 200, B: 160, A: 255},
		Key:    tv4p.Color{R: 150, G: 120, B: 80, A: 255},
	},
	{
		Keys:   []string{"concrete"},
		Normal: tv4p.Color{R: 170, G: 170, B: 170, A: 255},
		Key:    tv4p.Color{R: 100, G: 100, B: 100, A: 255},
	},
	{
		Keys:   []string{"rail"},
		Normal: tv4p.Color{R: 200, G: 70, B: 50, A: 255},
		Key:    tv4p.Color{R: 120, G: 30, B: 20, A: 255},
	},
	{
		Keys:   []string{"track"},
		Normal: tv4p.Color{R: 170, G: 80, B: 200, A: 255},
		Key:    tv4p.Color{R: 90, G: 40, B: 120, A: 255},
	},
	{
		Keys:   []string{"way"},
		Normal: tv4p.Color{R: 80, G: 200, B: 120, A: 255},
		Key:    tv4p.Color{R: 30, G: 120, B: 70, A: 255},
	},
	{
		Keys:   []string{"path"},
		Normal: tv4p.Color{R: 90, G: 180, B: 90, A: 255},
		Key:    tv4p.Color{R: 40, G: 110, B: 40, A: 255},
	},
	{
		Keys:   []string{"asf1"},
		Normal: tv4p.Color{R: 110, G: 125, B: 150, A: 255},
		Key:    tv4p.Color{R: 60, G: 80, B: 110, A: 255},
	},
	{
		Keys:   []string{"asf2"},
		Normal: tv4p.Color{R: 120, G: 115, B: 110, A: 255},
		Key:    tv4p.Color{R: 70, G: 70, B: 65, A: 255},
	},
	{
		Keys:   []string{"asf3"},
		Normal: tv4p.Color{R: 125, G: 105, B: 90, A: 255},
		Key:    tv4p.Color{R: 75, G: 60, B: 45, A: 255},
	},
	{
		Keys:   []string{"asf"},
		Normal: tv4p.Color{R: 95, G: 115, B: 140, A: 255},
		Key:    tv4p.Color{R: 50, G: 70, B: 100, A: 255},
	},
	{
		Keys:   []string{"city"},
		Normal: tv4p.Color{R: 230, G: 170, B: 70, A: 255},
		Key:    tv4p.Color{R: 170, G: 110, B: 25, A: 255},
	},
	{
		Keys:   []string{"grav"},
		Normal: tv4p.Color{R: 190, G: 145, B: 90, A: 255},
		Key:    tv4p.Color{R: 120, G: 80, B: 40, A: 255},
	},
	{
		Keys:   []string{"mud"},
		Normal: tv4p.Color{R: 140, G: 90, B: 55, A: 255},
		Key:    tv4p.Color{R: 90, G: 50, B: 25, A: 255},
	},
	{
		Keys:   []string{"quarry"},
		Normal: tv4p.Color{R: 210, G: 160, B: 80, A: 255},
		Key:    tv4p.Color{R: 140, G: 90, B: 35, A: 255},
	},
}
