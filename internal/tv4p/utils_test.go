package tv4p

import "testing"

func TestHash32Deterministic(t *testing.T) {
	t.Parallel()

	a := hash32("asf1_6")
	b := hash32("asf1_6")
	if a != b {
		t.Fatalf("hash not deterministic: %v vs %v", a, b)
	}

	c := hash32("asf1_7")
	if a == c {
		t.Fatalf("hash collision for different inputs")
	}
}

func TestWriteU32FromInt(t *testing.T) {
	t.Parallel()

	buf := make([]byte, 4)
	if err := writeU32FromInt(buf, 12345); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if readU32(buf) != 12345 {
		t.Fatalf("roundtrip mismatch: %v", readU32(buf))
	}

	if err := writeU32FromInt(buf, -1); err == nil {
		t.Fatalf("expected error for negative value")
	}
}
