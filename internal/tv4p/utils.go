package tv4p

import (
	"encoding/binary"
	"errors"

	"github.com/cespare/xxhash"
)

// readU16 reads a 16-bit integer from a byte slice.
func readU16(b []byte) uint16 {
	if len(b) < 2 {
		return 0
	}

	return uint16(b[0]) | uint16(b[1])<<8
}

// readU32 reads a 32-bit integer from a byte slice.
func readU32(b []byte) uint32 {
	if len(b) < 4 {
		return 0
	}

	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// boolByte builds a boolean byte from the configuration.
func boolByte(v bool) byte {
	if v {
		return 1
	}

	return 0
}

// hash32 builds a deterministic 32-bit hash.
func hash32(s string) uint32 {
	var buf [8]byte
	h := xxhash.Sum64String(s)

	binary.LittleEndian.PutUint64(buf[:], h)
	lo := binary.LittleEndian.Uint32(buf[:4])
	hi := binary.LittleEndian.Uint32(buf[4:])

	return lo ^ hi
}

// writeU16 writes a 16-bit integer to the configuration.
func writeU16(b []byte, v uint16) {
	if len(b) < 2 {
		return
	}

	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

// writeU16FromInt writes a 16-bit integer to the configuration.
func writeU16FromInt(b []byte, v int) error {
	if v < 0 || v > 0xFFFF {
		return errors.New("value out of uint16 range")
	}
	if len(b) < 2 {
		return errors.New("buffer too small for uint16")
	}

	b[0] = byte(v)
	b[1] = byte(v >> 8)

	return nil
}

// writeU32 writes a 32-bit integer to the configuration.
func writeU32(b []byte, v uint32) {
	if len(b) < 4 {
		return
	}

	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

// writeU32FromInt writes a 32-bit integer to the configuration.
func writeU32FromInt(b []byte, v int) error {
	if v < 0 || v > 0xFFFFFFFF {
		return errors.New("value out of uint32 range")
	}
	if len(b) < 4 {
		return errors.New("buffer too small for uint32")
	}

	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)

	return nil
}
