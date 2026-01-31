package roadparts

import (
	"testing"

	"github.com/woozymasta/tv4p-road-tool/internal/tv4p"
)

func TestMixColorsWeighted(t *testing.T) {
	t.Parallel()

	asf1, _, _ := Palette("asf1")
	city, _, _ := Palette("city")

	// asf1 + asf1 should equal asf1 (weighted)
	m1 := MixColors(asf1, asf1)
	if m1 != asf1 {
		t.Fatalf("asf1+asf1: got=%+v want=%+v", m1, asf1)
	}

	// asf1 + asf1 + city should be closer to asf1 than to city (because asf1 weight=2)
	m2 := MixColors(asf1, asf1, city)
	if dist(m2, asf1) >= dist(m2, city) {
		t.Fatalf("weighted mix not closer to asf1: mix=%+v asf1=%+v city=%+v", m2, asf1, city)
	}
}

func dist(a, b tv4p.Color) int {
	dr := int(a.R) - int(b.R)
	dg := int(a.G) - int(b.G)
	db := int(a.B) - int(b.B)
	if dr < 0 {
		dr = -dr
	}
	if dg < 0 {
		dg = -dg
	}
	if db < 0 {
		db = -db
	}
	return dr + dg + db
}
