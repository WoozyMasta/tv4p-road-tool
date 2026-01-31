package roadparts

import "testing"

func TestParseCrossroadBase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string
		ok   bool
		want CrossroadName
	}{
		{
			name: "t_basic",
			base: "kr_t_asf1_asf2",
			ok:   true,
			want: CrossroadName{Shape: CrossroadShapeT, AB: "asf1", C: "asf2"},
		},
		{
			name: "x_basic_implicit_d",
			base: "kr_x_city_city",
			ok:   true,
			want: CrossroadName{Shape: CrossroadShapeX, AB: "city", C: "city", D: "city"},
		},
		{
			name: "x_explicit_d",
			base: "kr_x_city_city_asf3",
			ok:   true,
			want: CrossroadName{Shape: CrossroadShapeX, AB: "city", C: "city", D: "asf3"},
		},
		{
			name: "reject_not_crossroad",
			base: "asf1_6",
			ok:   false,
		},
		{
			name: "reject_missing_parts",
			base: "kr_t_asf1_",
			ok:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ParseCrossroadBase(tt.base)
			if ok != tt.ok {
				t.Fatalf("ok=%v want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if got != tt.want {
				t.Fatalf("got=%+v want %+v", got, tt.want)
			}
		})
	}
}
