package roadparts

import "github.com/woozymasta/tv4p-road-tool/internal/tv4p"

// MixColors returns the weighted average of the provided colors.
// Weighting is achieved by passing the same color multiple times.
func MixColors(colors ...tv4p.Color) tv4p.Color {
	if len(colors) == 0 {
		return tv4p.Color{R: 0, G: 0, B: 0, A: 255}
	}

	var sr, sg, sb int
	for _, c := range colors {
		sr += int(c.R)
		sg += int(c.G)
		sb += int(c.B)
	}

	n := len(colors)
	return tv4p.Color{
		R: byte(clamp255((sr + n/2) / n)),
		G: byte(clamp255((sg + n/2) / n)),
		B: byte(clamp255((sb + n/2) / n)),
		A: 255,
	}
}

// DarkenColor shifts a color to a darker shade by multiplying RGB by factor.
// factor should be in range (0, 1].
func DarkenColor(c tv4p.Color, factor float64) tv4p.Color {
	if factor <= 0 {
		return tv4p.Color{R: 0, G: 0, B: 0, A: 255}
	}

	if factor > 1 {
		factor = 1
	}

	r := int(float64(c.R) * factor)
	g := int(float64(c.G) * factor)
	b := int(float64(c.B) * factor)

	return tv4p.Color{R: byte(clamp255(r)), G: byte(clamp255(g)), B: byte(clamp255(b)), A: 255}
}

func clamp255(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
