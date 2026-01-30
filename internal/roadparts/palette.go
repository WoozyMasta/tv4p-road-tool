package roadparts

import (
	"encoding/binary"
	"strings"

	"github.com/cespare/xxhash"

	"github.com/woozymasta/tv4p-road-tool/internal/tv4p"
)

// Palette returns the color palette for a road part name.
func Palette(name string) (tv4p.Color, tv4p.Color, bool) {
	name = strings.ToLower(name)
	shiftBlue := strings.Contains(name, "sakhal")
	shiftGreen := strings.Contains(name, "enoch")

	for _, rule := range paletteRules {
		if rule.matches(name) {
			normal, key := applyWorldTint(rule.Normal, rule.Key, shiftBlue, shiftGreen)
			return normal, key, true
		}
	}

	normal := hashColor(name)
	key := darkenAndSaturate(normal, 0.7, 1.25)
	normal, key = applyWorldTint(normal, key, shiftBlue, shiftGreen)

	return normal, key, true
}

// matches checks if a name matches a palette rule.
func (r paletteRule) matches(name string) bool {
	for _, k := range r.Keys {
		if strings.Contains(name, k) {
			return true
		}
	}

	return false
}

// hashColor hashes a name to a color.
func hashColor(name string) tv4p.Color {
	h64 := xxhash.Sum64String(name)
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], h64)
	lo := binary.LittleEndian.Uint32(buf[:4])
	hi := binary.LittleEndian.Uint32(buf[4:])
	h := lo ^ hi
	r := byte(60 + (h&0xff)%160)
	g := byte(60 + ((h>>8)&0xff)%160)
	b := byte(60 + ((h>>16)&0xff)%160)
	avg := (int(r) + int(g) + int(b)) / 3
	r = clampByte(avg + int(float64(int(r)-avg)*1.2))
	g = clampByte(avg + int(float64(int(g)-avg)*1.2))
	b = clampByte(avg + int(float64(int(b)-avg)*1.2))

	return tv4p.Color{R: r, G: g, B: b, A: 255}
}

// darkenAndSaturate darkens and saturates a color.
func darkenAndSaturate(c tv4p.Color, darken float64, sat float64) tv4p.Color {
	avg := (int(c.R) + int(c.G) + int(c.B)) / 3
	r := clampByte(int(float64(int(c.R)-avg)*sat) + avg)
	g := clampByte(int(float64(int(c.G)-avg)*sat) + avg)
	b := clampByte(int(float64(int(c.B)-avg)*sat) + avg)
	r = clampByte(int(float64(r) * darken))
	g = clampByte(int(float64(g) * darken))
	b = clampByte(int(float64(b) * darken))

	return tv4p.Color{R: r, G: g, B: b, A: 255}
}

// applyWorldTint applies a world tint to a color.
func applyWorldTint(normal tv4p.Color, key tv4p.Color, shiftBlue bool, shiftGreen bool) (tv4p.Color, tv4p.Color) {
	if shiftBlue {
		normal = shiftChannel(normal, 0, 25)
		key = shiftChannel(key, 0, 18)
	}
	if shiftGreen {
		normal = shiftChannel(normal, 20, 0)
		key = shiftChannel(key, 14, 0)
	}

	return normal, key
}

// shiftChannel shifts a channel of a color.
func shiftChannel(c tv4p.Color, dg int, db int) tv4p.Color {
	r := clampByte(int(c.R))
	g := clampByte(int(c.G) + dg)
	b := clampByte(int(c.B) + db)

	return tv4p.Color{R: r, G: g, B: b, A: 255}
}

// clampByte clamps a byte value.
func clampByte(v int) byte {
	if v < 40 {
		return 40
	}
	if v > 220 {
		return 220
	}

	return byte(v)
}
