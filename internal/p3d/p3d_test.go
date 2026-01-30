package p3d

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsMLOD(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		hdr  string
		ok   bool
		kind string
	}{
		{name: "mlod", hdr: "MLOD", ok: true, kind: "MLOD"},
		{name: "odol", hdr: "ODOL", ok: false, kind: "ODOL"},
		{name: "unk", hdr: "XXXX", ok: false, kind: "UNKNOWN"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			localPath := filepath.Join(t.TempDir(), "test.p3d")
			if err := os.WriteFile(localPath, []byte(tt.hdr), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}

			ok, kind, err := IsMLOD(localPath)
			if err != nil {
				t.Fatalf("IsMLOD error: %v", err)
			}
			if ok != tt.ok {
				t.Fatalf("ok=%v want %v", ok, tt.ok)
			}
			if kind != tt.kind {
				t.Fatalf("kind=%q want %q", kind, tt.kind)
			}
		})
	}
}
