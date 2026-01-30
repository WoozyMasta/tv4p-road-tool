package roadparts

import "testing"

func TestPaletteBasic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ok   bool
	}{
		{name: "asf1", ok: true},
		{name: "weird_type_123", ok: true},
		{name: "sakhal_asf2", ok: true},
		{name: "enoch_city", ok: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			normal, key, ok := Palette(tt.name)
			if ok != tt.ok {
				t.Fatalf("ok=%v want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if normal.A != 255 || key.A != 255 {
				t.Fatalf("alpha not 255: normal=%d key=%d", normal.A, key.A)
			}
			if normal == key {
				t.Fatalf("expected key to differ from normal")
			}
		})
	}
}
