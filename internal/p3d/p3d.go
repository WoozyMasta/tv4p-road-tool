// Package p3d provides functions to check P3D file headers.
package p3d

import (
	"io"
	"os"
)

// IsMLOD reads the P3D header and reports whether it is an MLOD file.
// It returns ok=true for MLOD, ok=false otherwise, and the detected kind string.
func IsMLOD(path string) (ok bool, kind string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return false, "", err
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	var hdr [4]byte
	n, err := io.ReadFull(f, hdr[:])
	if err != nil {
		return false, "", err
	}
	if n != 4 {
		return false, "", io.ErrUnexpectedEOF
	}

	switch string(hdr[:]) {
	case "MLOD":
		return true, "MLOD", nil
	case "ODOL":
		return false, "ODOL", nil
	default:
		return false, "UNKNOWN", nil
	}
}
