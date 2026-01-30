package roadparts

import "testing"

func TestParseBase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		base     string
		ok       bool
		kind     Kind
		typeName string
		partName string
	}{
		{
			name:     "straight",
			base:     "asf1_6",
			ok:       true,
			kind:     Straight,
			typeName: "asf1",
			partName: "asf1_6",
		},
		{
			name:     "corner",
			base:     "asf1_7 100",
			ok:       true,
			kind:     Corner,
			typeName: "asf1",
			partName: "asf1_7 100",
		},
		{
			name:     "terminator",
			base:     "asf1_6konec",
			ok:       true,
			kind:     Terminator,
			typeName: "asf1",
			partName: "asf1_6konec",
		},
		{
			name:     "crosswalk",
			base:     "asf1_6_crosswalk",
			ok:       true,
			kind:     Crosswalk,
			typeName: "asf1",
			partName: "asf1_6_crosswalk",
		},
		{
			name:     "crossroad",
			base:     "kr_t_asf1_city",
			ok:       true,
			kind:     Crossroad,
			typeName: "crossroad",
			partName: "kr_t_asf1_city",
		},
		{
			name: "reject",
			base: "asf1_",
			ok:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ParseBase(tt.base)
			if ok != tt.ok {
				t.Fatalf("ok=%v want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if got.Kind != tt.kind {
				t.Fatalf("kind=%v want %v", got.Kind, tt.kind)
			}
			if got.TypeName != tt.typeName {
				t.Fatalf("type=%q want %q", got.TypeName, tt.typeName)
			}
			if got.Name != tt.partName {
				t.Fatalf("name=%q want %q", got.Name, tt.partName)
			}
		})
	}
}
