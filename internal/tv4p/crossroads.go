package tv4p

import (
	"encoding/hex"
	"strings"
)

// ParseRoadToolConfig extracts both road types (0x88) and crossroad definitions (0x89/0x8A)
// into a single config structure.
func ParseRoadToolConfig(data []byte) (RoadConfig, error) {
	rtBlock, err := ParseRoadTypes(data)
	if err != nil {
		return RoadConfig{}, err
	}

	cfg := RoadConfig{Types: rtBlock.Types}

	afterRoadTypes := rtBlock.Start + 7 + rtBlock.ListLen
	crDefs, ok := findTaggedListAfter(data, afterRoadTypes, 0x89, validateCrossroadDefs)
	if !ok {
		// no crossroads in file (or not found near Road Tool region)
		return cfg, nil
	}

	afterDefs := crDefs.Start + crDefs.FieldLen
	crLinks, _ := findTaggedListAfter(data, afterDefs, 0x8A, validateCrossroadLinks)

	linksByModel := map[string]Entry{}
	if crLinks.Found {
		for _, e := range crLinks.Entries {
			if p := entryString(e, 0x91); p != "" {
				linksByModel[p] = e
			}
		}
	}

	for _, e := range crDefs.Entries {
		name := entryString(e, 0x33)
		model := entryString(e, 0x7C)
		color := entryColor(e, 0x73)
		colorCustom := entryByte(e, 0x71) != 0

		conns := CrossroadConnections{}
		aIdx, aOK := entryU32(e, 0x84)
		bIdx, bOK := entryU32(e, 0x85)
		cIdx, cOK := entryU32(e, 0x86)
		dIdx, dOK := entryU32(e, 0x87)

		if aOK {
			conns.A = idxToRoadType(cfg.Types, aIdx)
		}
		if bOK {
			conns.B = idxToRoadType(cfg.Types, bIdx)
		}
		if cOK {
			conns.C = idxToRoadType(cfg.Types, cIdx)
		}
		if dOK {
			conns.D = idxToRoadType(cfg.Types, dIdx)
		}

		// Fallback: if indices are missing/out of range, derive from name semantics.
		if conns.A == "" && conns.B == "" && conns.C == "" && conns.D == "" {
			ab, c, d, shapeOK := parseCrossroadNameTypes(name)
			if shapeOK {
				conns.A = ab
				conns.B = ab
				conns.C = c
				if d != "" {
					conns.D = d
				}
			}
		}

		cr := CrossroadType{
			Name:        name,
			Model:       model,
			Color:       color,
			ColorCustom: colorCustom,
			Connections: conns,
			TV4PDef:     entryToRaw(e),
		}

		if link, ok := linksByModel[model]; ok {
			cr.TV4PLink = entryToRaw(link)
		}

		cfg.CrossroadTypes = append(cfg.CrossroadTypes, cr)
	}

	// Derive "defaults" from list order for convenience:
	// in many observed projects TB effectively uses 0x89[roadTypeIndex] as a fallback.
	// We mark crossroad_types[i].default = road_types[i].name when it matches.
	limit := len(cfg.Types)
	if limit > len(cfg.CrossroadTypes) {
		limit = len(cfg.CrossroadTypes)
	}

	for i := 0; i < limit; i++ {
		rt := cfg.Types[i].Name
		if rt == "" {
			continue
		}
		if cfg.CrossroadTypes[i].Default != "" {
			continue
		}

		if crossroadHasRoadType(cfg.CrossroadTypes[i], rt) {
			cfg.CrossroadTypes[i].Default = rt
		}
	}

	return cfg, nil
}

// crossroadHasRoadType checks if a crossroad type has a specific road type in its connections.
// Example: `kr_t_asf1_asf2` has `asf1` in both `A` and `B` connections.
func crossroadHasRoadType(cr CrossroadType, rtName string) bool {
	want := strings.ToLower(rtName)
	if want == "" {
		return false
	}

	sides := []string{cr.Connections.A, cr.Connections.B, cr.Connections.C, cr.Connections.D}
	for _, s := range sides {
		if strings.ToLower(s) == want {
			return true
		}
	}

	return false
}

// taggedList represents a tagged list in a tv4p file.
type taggedList struct {
	Entries  []Entry // the entries in the list
	Start    int     // the start offset of the list
	ListLen  int     // the length of the list
	FieldLen int     // the length of the list fields
	Count    uint32  // the count of the list
	Found    bool    // true if the list was found
	Tag      byte    // the tag of the list
}

// listValidator is a function that validates a list of entries.
type listValidator func(entries []Entry) bool

// findTaggedListAfter finds a tagged list after a given offset.
func findTaggedListAfter(data []byte, from int, tag byte, validate listValidator) (taggedList, bool) {
	if from < 0 {
		from = 0
	}
	if from >= len(data) {
		return taggedList{}, false
	}

	pat := []byte{tag, 0x00, 0x0C}
	for i := from; i+11 <= len(data); i++ {
		if data[i] != pat[0] || data[i+1] != pat[1] || data[i+2] != pat[2] {
			continue
		}

		listLen := int(readU32(data[i+3:]))
		count := readU32(data[i+7:])
		if listLen < 4 {
			continue
		}

		entriesLen := listLen - 4
		entriesStart := i + 11
		if entriesStart+entriesLen > len(data) {
			continue
		}

		entries, ok := parseEntries(data, entriesStart, entriesLen, int(count), 0)
		if !ok {
			continue
		}

		if validate != nil && !validate(entries) {
			continue
		}

		fieldLen := 11 + entriesLen
		return taggedList{
			Found:    true,
			Tag:      tag,
			Start:    i,
			ListLen:  listLen,
			Count:    count,
			FieldLen: fieldLen,
			Entries:  entries,
		}, true
	}

	return taggedList{}, false
}

// validateCrossroadDefs validates a list of crossroad definitions.
func validateCrossroadDefs(entries []Entry) bool {
	for _, e := range entries {
		if e.TypeID != 0x17 {
			return false
		}
		if entryString(e, 0x33) == "" {
			return false
		}
		if entryString(e, 0x7C) == "" {
			return false
		}
	}

	return true
}

// validateCrossroadLinks validates a list of crossroad links.
func validateCrossroadLinks(entries []Entry) bool {
	for _, e := range entries {
		if e.TypeID != 0x1A {
			return false
		}

		// Must contain model path reference.
		if entryString(e, 0x91) == "" {
			return false
		}
	}

	return true
}

// entryString extracts a string from an entry.
func entryString(e Entry, tag byte) string {
	for _, f := range e.Fields {
		if f.Tag == tag && f.Type == 0x0B {
			return string(f.Raw)
		}
	}

	return ""
}

// entryColor extracts a color from an entry.
func entryColor(e Entry, tag byte) Color {
	for _, f := range e.Fields {
		if f.Tag == tag && f.Type == 0x08 && len(f.Raw) >= 4 {
			return Color{R: f.Raw[0], G: f.Raw[1], B: f.Raw[2], A: f.Raw[3]}
		}
	}

	return Color{}
}

// entryByte extracts a byte from an entry.
func entryByte(e Entry, tag byte) byte {
	for _, f := range e.Fields {
		if f.Tag == tag && f.Type == 0x09 && len(f.Raw) >= 1 {
			return f.Raw[0]
		}
	}

	return 0
}

// entryU32 extracts a 32-bit unsigned integer from an entry.
func entryU32(e Entry, tag byte) (uint32, bool) {
	for _, f := range e.Fields {
		if f.Tag != tag {
			continue
		}

		// Crossroad indices observed as type 0x05 u32.
		if (f.Type == 0x05 || f.Type == 0x0D) && len(f.Raw) >= 4 {
			return readU32(f.Raw), true
		}
	}

	return 0, false
}

// idxToRoadType converts a road type index to a road type name.
func idxToRoadType(types []RoadType, idx uint32) string {
	if idx == 0xFFFFFFFF {
		return ""
	}
	// Avoid uint32(len(types)) conversion (gosec G115).
	if uint64(idx) >= uint64(len(types)) {
		return ""
	}
	return types[idx].Name
}

// parseCrossroadNameTypes parses a crossroad name into its components.
func parseCrossroadNameTypes(name string) (ab string, c string, d string, ok bool) {
	// T-junction
	if strings.HasPrefix(name, "kr_t_") {
		rest := strings.TrimPrefix(name, "kr_t_")
		parts := strings.Split(rest, "_")
		if len(parts) != 2 {
			return "", "", "", false
		}

		return parts[0], parts[1], "", true
	}

	// X-junction
	if strings.HasPrefix(name, "kr_x_") {
		rest := strings.TrimPrefix(name, "kr_x_")
		parts := strings.Split(rest, "_")
		if len(parts) < 2 {
			return "", "", "", false
		}

		ab = parts[0]
		c = parts[1]
		if len(parts) > 2 {
			d = strings.Join(parts[2:], "_")
		} else {
			d = c
		}

		return ab, c, d, true
	}

	return "", "", "", false
}

// entryToRaw converts an entry to an EntryRaw.
func entryToRaw(e Entry) *EntryRaw {
	raw := &EntryRaw{
		Type: e.TypeID,
		ID:   e.ID,
	}

	for _, f := range e.Fields {
		fr := FieldRaw{
			Tag:  f.Tag,
			Type: f.Type,
		}

		if len(f.Raw) > 0 {
			fr.Raw = hex.EncodeToString(f.Raw)
		}
		if len(f.List) > 0 {
			for _, le := range f.List {
				fr.List = append(fr.List, *entryToRaw(le))
			}
		}

		raw.Fields = append(raw.Fields, fr)
	}

	return raw
}
