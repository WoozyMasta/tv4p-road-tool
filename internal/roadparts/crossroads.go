package roadparts

import "strings"

// CrossroadShape describes the crossroad model family.
type CrossroadShape int

const (
	// CrossroadShapeUnknown is an invalid/unknown crossroad shape.
	CrossroadShapeUnknown CrossroadShape = iota
	// CrossroadShapeT represents `kr_t_*` models.
	CrossroadShapeT
	// CrossroadShapeX represents `kr_x_*` models.
	CrossroadShapeX
)

// CrossroadName is the semantic meaning encoded in a crossroad model name.
//
// Examples:
// - kr_t_asf1_asf2 -> ShapeT, AB=asf1, C=asf2
// - kr_x_city_city -> ShapeX, AB=city, C=city, D=city (implicit)
// - kr_x_city_city_asf3 -> ShapeX, AB=city, C=city, D=asf3
type CrossroadName struct {
	AB    string         // main road through the crossroad
	C     string         // the branch
	D     string         // for X-shape, D is either explicit or equals C
	Shape CrossroadShape // for T-shape, D is not used
}

// ParseCrossroadBase parses `kr_t_*` / `kr_x_*` naming into a semantic structure.
func ParseCrossroadBase(base string) (CrossroadName, bool) {
	if strings.HasPrefix(base, "kr_t_") {
		rest := strings.TrimPrefix(base, "kr_t_")
		parts := strings.Split(rest, "_")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return CrossroadName{}, false
		}
		return CrossroadName{Shape: CrossroadShapeT, AB: parts[0], C: parts[1]}, true
	}

	if strings.HasPrefix(base, "kr_x_") {
		rest := strings.TrimPrefix(base, "kr_x_")
		parts := strings.Split(rest, "_")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return CrossroadName{}, false
		}

		out := CrossroadName{Shape: CrossroadShapeX, AB: parts[0], C: parts[1]}
		if len(parts) >= 3 {
			out.D = strings.Join(parts[2:], "_")
		} else {
			out.D = out.C
		}
		if out.D == "" {
			out.D = out.C
		}
		return out, true
	}

	return CrossroadName{}, false
}
