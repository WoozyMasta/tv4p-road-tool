package tv4p

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
)

// PatchRoadTypes rewrites the road types block using the provided config.
func PatchRoadTypes(data []byte, cfg RoadConfig) ([]byte, error) {
	block, err := ParseRoadTypes(data)
	if err != nil {
		return nil, err
	}

	existingIDs := collectEntryIDs(data)
	listBytes, err := buildRoadTypesList(cfg, existingIDs)
	if err != nil {
		return nil, err
	}

	newListLen := len(listBytes) + 4
	entriesStart := block.EntriesStart
	entriesEnd := entriesStart + block.EntriesLen
	if entriesEnd > len(data) || entriesStart < 0 {
		return nil, errors.New("invalid entries range")
	}

	out := make([]byte, 0, len(data)-(block.EntriesLen)+len(listBytes))
	out = append(out, data[:entriesStart]...)
	out = append(out, listBytes...)
	out = append(out, data[entriesEnd:]...)

	// update listLen and count in header
	if err := writeU32FromInt(out[block.Start+3:], newListLen); err != nil {
		return nil, err
	}
	if err := writeU32FromInt(out[block.Start+7:], len(cfg.Types)); err != nil {
		return nil, err
	}

	if delta := newListLen - block.ListLen; delta != 0 {
		if err := adjustOffsetsByTag(out, 0x18, 0x0D, delta); err != nil {
			return nil, err
		}
		if err := adjustOffsetsByTag(out, 0x3E, 0x0D, delta); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// buildRoadTypesList builds the road types list from the configuration.
func buildRoadTypesList(cfg RoadConfig, existingIDs map[uint32]struct{}) ([]byte, error) {
	alloc := newIDAllocator(cfg, existingIDs)
	var entries [][]byte
	for _, rt := range cfg.Types {
		entry, err := buildRoadTypeEntry(rt, alloc)
		if err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	return buildList(entries), nil
}

// buildRoadTypeEntry builds a single road type entry from the configuration.
func buildRoadTypeEntry(rt RoadType, alloc *idAllocator) ([]byte, error) {
	var fields [][]byte
	nameField, err := fieldString(0x33, rt.Name)
	if err != nil {
		return nil, err
	}

	fields = append(fields, nameField)
	fields = append(fields, fieldByte(0x71, boolByte(rt.KeyCustom)))
	fields = append(fields, fieldByte(0x72, boolByte(rt.NormalCustom)))
	fields = append(fields, fieldColor(0x73, rt.NormalColor, rt.NormalCustom))
	fields = append(fields, fieldColor(0x74, rt.KeyColor, rt.KeyCustom))
	fields = append(fields, fieldByte(0x75, 0))
	fields = append(fields, fieldBytes(0x76, make([]byte, 8)))
	fields = append(fields, fieldBytes(0x77, make([]byte, 8)))
	straight, err := buildPartsList(rt.StraightParts, 0x13, true, alloc)
	if err != nil {
		return nil, err
	}

	straightField, err := fieldList(0x78, straight)
	if err != nil {
		return nil, err
	}
	fields = append(fields, straightField)
	corners, err := buildPartsList(rt.CornerParts, 0x14, false, alloc)
	if err != nil {
		return nil, err
	}

	cornerField, err := fieldList(0x79, corners)
	if err != nil {
		return nil, err
	}

	fields = append(fields, cornerField)
	emptyField, err := fieldList(0x7A, nil)
	if err != nil {
		return nil, err
	}

	fields = append(fields, emptyField)
	terminators, err := buildPartsList(rt.TerminatorPart, 0x16, false, alloc)
	if err != nil {
		return nil, err
	}

	terminatorField, err := fieldList(0x7B, terminators)
	if err != nil {
		return nil, err
	}

	fields = append(fields, terminatorField)

	entryType := rt.Type
	if entryType == 0 {
		entryType = 0x12
	}
	entryID := alloc.useOrDeterministic(rt.ID, "rt|"+strings.ToLower(rt.Name))

	return buildEntry(entryType, entryID, fields)
}

// buildPartsList builds the parts list from the configuration.
func buildPartsList(parts []RoadPart, defaultType uint16, includeFlag bool, alloc *idAllocator) ([][]byte, error) {
	var entries [][]byte
	for _, p := range parts {
		var fields [][]byte
		nameField, err := fieldString(0x33, p.Name)
		if err != nil {
			return nil, err
		}

		pathField, err := fieldString(0x7C, p.Path)
		if err != nil {
			return nil, err
		}

		fields = append(fields, nameField, pathField)
		if includeFlag {
			fields = append(fields, fieldByte(0x7D, 0))
		}

		fields = append(fields, fieldBytes(0x7E, make([]byte, 8)))
		typ := p.Type
		if typ == 0 {
			typ = defaultType
		}

		seed := buildPartSeed(typ, p.Name, p.Path)
		entryID := alloc.useOrDeterministic(p.ID, seed)
		entry, err := buildEntry(typ, entryID, fields)
		if err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// buildEntry builds a single entry from the configuration.
func buildEntry(typeID uint16, id uint32, fields [][]byte) ([]byte, error) {
	body := make([]byte, 0, 64)
	tmp := make([]byte, 2)
	writeU16(tmp, typeID)

	body = append(body, tmp...)
	tmp4 := make([]byte, 4)
	writeU32(tmp4, id)

	body = append(body, tmp4...)
	for _, f := range fields {
		body = append(body, f...)
	}
	out := make([]byte, 0, len(body)+7)
	out = append(out, 0x06, 0x00, 0x0D)

	if err := writeU32FromInt(tmp4, len(body)); err != nil {
		return nil, err
	}

	out = append(out, tmp4...)
	out = append(out, body...)

	return out, nil
}

// buildList builds a list from the configuration.
func buildList(entries [][]byte) []byte {
	total := 0
	for _, e := range entries {
		total += len(e)
	}

	out := make([]byte, 0, total)
	for _, e := range entries {
		out = append(out, e...)
	}

	return out
}

// idAllocator allocates IDs for new entries.
type idAllocator struct {
	used map[uint32]struct{}
}

// newIDAllocator creates a new ID allocator.
func newIDAllocator(cfg RoadConfig, existing map[uint32]struct{}) *idAllocator {
	used := map[uint32]struct{}{}
	for id := range existing {
		if id != 0 {
			used[id] = struct{}{}
		}
	}

	for _, rt := range cfg.Types {
		if rt.ID != 0 {
			used[rt.ID] = struct{}{}
		}
		for _, p := range rt.StraightParts {
			if p.ID != 0 {
				used[p.ID] = struct{}{}
			}
		}
		for _, p := range rt.CornerParts {
			if p.ID != 0 {
				used[p.ID] = struct{}{}
			}
		}
		for _, p := range rt.TerminatorPart {
			if p.ID != 0 {
				used[p.ID] = struct{}{}
			}
		}
	}

	return &idAllocator{used: used}
}

// useOrDeterministic allocates an ID or uses a deterministic ID.
func (a *idAllocator) useOrDeterministic(id uint32, seed string) uint32 {
	if id != 0 {
		a.used[id] = struct{}{}
		return id
	}
	base := strings.ToLower(seed)
	for i := 0; ; i++ {
		s := base
		if i > 0 {
			s = base + "#" + strconv.Itoa(i)
		}

		h := hash32(s)
		if h == 0 {
			continue
		}

		if _, ok := a.used[h]; ok {
			continue
		}

		a.used[h] = struct{}{}
		return h
	}
}

// buildPartSeed builds a seed for a part.
func buildPartSeed(typ uint16, name string, path string) string {
	var b strings.Builder
	b.Grow(len(name) + len(path) + 16)
	b.WriteString("part|")
	b.WriteString(strconv.Itoa(int(typ)))
	b.WriteByte('|')
	b.WriteString(name)
	b.WriteByte('|')
	b.WriteString(path)

	return b.String()
}

// collectEntryIDs collects the IDs of all entries in the data.
func collectEntryIDs(data []byte) map[uint32]struct{} {
	used := map[uint32]struct{}{}
	for i := 0; i+7 < len(data); i++ {
		if data[i] != 0x06 || data[i+1] != 0x00 || data[i+2] != 0x0D {
			continue
		}

		bodyLen := int(readU32(data[i+3:]))
		if bodyLen < 6 {
			continue
		}

		bodyStart := i + 7
		bodyEnd := bodyStart + bodyLen
		if bodyEnd > len(data) {
			continue
		}

		id := readU32(data[bodyStart+2:])
		if id != 0 {
			used[id] = struct{}{}
		}
	}

	return used
}

// adjustOffsetsByTag adjusts the offsets of all entries with the given tag and type.
func adjustOffsetsByTag(data []byte, tag byte, typ byte, delta int) error {
	if delta == 0 {
		return nil
	}

	if delta < -0x7fffffff || delta > 0x7fffffff {
		return errors.New("offset delta is out of range")
	}

	pattern := []byte{tag, 0x00, typ}
	off := 0
	count := 0
	for {
		idx := bytes.Index(data[off:], pattern)
		if idx < 0 {
			break
		}

		pos := off + idx
		if pos+7 <= len(data) {
			count++
			off = pos + 1
			continue
		}

		break
	}

	if count != 1 {
		return errors.New("offset tag count mismatch")
	}

	pos := bytes.Index(data, pattern)
	if pos < 0 || pos+7 > len(data) {
		return errors.New("offset tag not found")
	}

	valPos := pos + 3
	cur := int(readU32(data[valPos:]))
	cur += delta
	if cur < 0 {
		cur = 0
	}
	if err := writeU32FromInt(data[valPos:], cur); err != nil {
		return err
	}

	return nil
}

// fieldString builds a string field from the configuration.
func fieldString(tag byte, s string) ([]byte, error) {
	b := []byte(s)
	out := make([]byte, 0, len(b)+5)
	out = append(out, tag, 0x00, 0x0B)
	tmp := make([]byte, 2)
	if err := writeU16FromInt(tmp, len(b)); err != nil {
		return nil, err
	}

	out = append(out, tmp...)
	out = append(out, b...)

	return out, nil
}

// fieldByte builds a byte field from the configuration.
func fieldByte(tag byte, v byte) []byte {
	return []byte{tag, 0x00, 0x09, v}
}

// fieldColor builds a color field from the configuration.
func fieldColor(tag byte, c Color, custom bool) []byte {
	if !custom {
		return []byte{tag, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00}
	}

	return []byte{tag, 0x00, 0x08, c.R, c.G, c.B, 0xFF}
}

// fieldBytes builds a bytes field from the configuration.
func fieldBytes(tag byte, b []byte) []byte {
	out := make([]byte, 0, len(b)+3)
	out = append(out, tag, 0x00, 0x14)
	out = append(out, b...)

	return out
}

// fieldList builds a list field from the configuration.
func fieldList(tag byte, entries [][]byte) ([]byte, error) {
	listBytes := buildList(entries)
	listLen := len(listBytes) + 4
	out := make([]byte, 0, len(listBytes)+11)
	out = append(out, tag, 0x00, 0x0C)
	tmp := make([]byte, 4)
	if err := writeU32FromInt(tmp, listLen); err != nil {
		return nil, err
	}

	out = append(out, tmp...)
	if err := writeU32FromInt(tmp, len(entries)); err != nil {
		return nil, err
	}

	out = append(out, tmp...)
	out = append(out, listBytes...)

	return out, nil
}
