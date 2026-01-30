package tv4p

import (
	"errors"
	"fmt"
	"strings"
)

// ParseRoadTypes parses the road types block from a tv4p file.
func ParseRoadTypes(data []byte) (*RoadTypesBlock, error) {
	meta, count, entries, err := findRoadTypesList(data)
	if err != nil {
		return nil, err
	}

	if len(entries) != int(count) {
		return nil, fmt.Errorf("entry count mismatch: header=%d parsed=%d", count, len(entries))
	}

	var out []RoadType
	for _, e := range entries {
		rt := RoadType{}
		rt.ID = e.ID
		rt.Type = e.TypeID
		for _, f := range e.Fields {
			switch f.Tag {
			case 0x33: // name
				rt.Name = string(f.Raw)
			case 0x71: // q
				rt.KeyCustom = len(f.Raw) > 0 && f.Raw[0] != 0
			case 0x72: // r
				rt.NormalCustom = len(f.Raw) > 0 && f.Raw[0] != 0
			case 0x73: // s
				if len(f.Raw) >= 4 {
					rt.NormalColor = Color{f.Raw[0], f.Raw[1], f.Raw[2], f.Raw[3]}
				}
			case 0x74: // t
				if len(f.Raw) >= 4 {
					rt.KeyColor = Color{f.Raw[0], f.Raw[1], f.Raw[2], f.Raw[3]}
				}
			case 0x78: // x: straight list
				rt.StraightParts = extractParts(f.List)
			case 0x79: // y: corner list
				rt.CornerParts = extractParts(f.List)
			case 0x7B: // { : terminator list
				rt.TerminatorPart = extractParts(f.List)
			}
		}
		out = append(out, rt)
	}

	return &RoadTypesBlock{
		Start:        meta.Start,
		Count:        count,
		ListLen:      meta.ListLen,
		EntriesStart: meta.EntriesStart,
		EntriesLen:   meta.EntriesLen,
		Entries:      entries,
		Types:        out,
	}, nil
}

// extractParts extracts the parts from a list of entries.
func extractParts(list []Entry) []RoadPart {
	var parts []RoadPart
	for _, e := range list {
		p := RoadPart{
			ID:   e.ID,
			Type: e.TypeID,
		}

		for _, f := range e.Fields {
			switch f.Tag {
			case 0x33:
				p.Name = string(f.Raw)
			case 0x7C:
				p.Path = string(f.Raw)
			}
		}

		if p.Name != "" || p.Path != "" {
			parts = append(parts, p)
		}
	}

	return parts
}

// findRoadTypesList finds the road types list in a byte slice.
func findRoadTypesList(data []byte) (roadTypesMeta, uint32, []Entry, error) {
	for i := 0; i+11 < len(data); i++ {
		if data[i] != 0x88 || data[i+1] != 0x00 || data[i+2] != 0x0C {
			continue
		}

		listLen := int(readU32(data[i+3:]))
		countU32 := readU32(data[i+7:])
		if countU32 > (^uint32(0) >> 1) {
			continue
		}

		count := int(countU32)
		if listLen < 4 {
			continue
		}

		entriesLen := listLen - 4
		pos := i + 11
		entries, ok := parseEntries(data, pos, entriesLen, count, 0)
		if !ok {
			continue
		}

		found := false
		for _, e := range entries {
			if entryHasRoadPath(e) || entryHasRoadLists(e) {
				found = true
				break
			}
		}

		if found {
			return roadTypesMeta{
				Start:        i,
				ListLen:      listLen,
				EntriesStart: pos,
				EntriesLen:   entriesLen,
			}, countU32, entries, nil
		}
	}

	return roadTypesMeta{}, 0, nil, errors.New("road types list not found")
}

// entryHasRoadPath checks if an entry has a road path.
func entryHasRoadPath(e Entry) bool {
	for _, f := range e.Fields {
		if f.Tag == 0x7C && looksLikeRoadPath(string(f.Raw)) {
			return true
		}

		for _, sub := range f.List {
			for _, sf := range sub.Fields {
				if sf.Tag == 0x7C && looksLikeRoadPath(string(sf.Raw)) {
					return true
				}
			}
		}
	}

	return false
}

func entryHasRoadLists(e Entry) bool {
	for _, f := range e.Fields {
		switch f.Tag {
		case 0x78, 0x79, 0x7B:
			return true
		}
	}

	return false
}

func looksLikeRoadPath(p string) bool {
	p = strings.ToLower(p)
	p = strings.ReplaceAll(p, "/", "\\")
	return strings.Contains(p, ".p3d")
}

// parseEntries parses a list of entries from a byte slice.
func parseEntries(data []byte, pos int, listLen int, count int, baseOffset int) ([]Entry, bool) {
	end := pos + listLen
	if end > len(data) {
		return nil, false
	}

	var entries []Entry
	for pos+7 <= end && len(entries) < count {
		if data[pos] != 0x06 || data[pos+1] != 0x00 || data[pos+2] != 0x0D {
			return nil, false
		}

		bodyLen := int(readU32(data[pos+3:]))
		bodyStart := pos + 7
		bodyEnd := bodyStart + bodyLen
		if bodyEnd > end {
			return nil, false
		}

		ent, ok := parseEntry(data[bodyStart:bodyEnd], baseOffset+bodyStart)
		if !ok {
			return nil, false
		}

		entries = append(entries, ent)
		pos = bodyEnd
	}

	return entries, true
}

// parseEntry parses a single entry from a byte slice.
func parseEntry(body []byte, absStart int) (Entry, bool) {
	if len(body) < 6 {
		return Entry{}, false
	}

	ent := Entry{
		TypeID:   readU16(body),
		ID:       readU32(body[2:]),
		Offset:   absStart,
		IDOffset: absStart + 2,
	}

	pos := 6
	for pos+3 <= len(body) {
		tag := body[pos]
		if body[pos+1] != 0x00 {
			return Entry{}, false
		}

		typ := body[pos+2]
		pos += 3

		switch typ {
		case 0x0B: // string
			if pos+2 > len(body) {
				return Entry{}, false
			}

			ln := int(readU16(body[pos:]))
			pos += 2
			if pos+ln > len(body) {
				return Entry{}, false
			}

			ent.Fields = append(ent.Fields, Field{Tag: tag, Type: typ, Raw: body[pos : pos+ln]})
			pos += ln

		case 0x09: // byte
			if pos+1 > len(body) {
				return Entry{}, false
			}

			ent.Fields = append(ent.Fields, Field{Tag: tag, Type: typ, Raw: body[pos : pos+1]})
			pos++

		case 0x08: // color
			if pos+4 > len(body) {
				return Entry{}, false
			}

			ent.Fields = append(ent.Fields, Field{Tag: tag, Type: typ, Raw: body[pos : pos+4]})
			pos += 4

		case 0x14: // bytes
			if pos+8 > len(body) {
				return Entry{}, false
			}

			ent.Fields = append(ent.Fields, Field{Tag: tag, Type: typ, Raw: body[pos : pos+8]})
			pos += 8

		case 0x0C: // list
			if pos+8 > len(body) {
				return Entry{}, false
			}

			listLen := int(readU32(body[pos:]))
			count := int(readU32(body[pos+4:]))
			pos += 8
			if listLen < 4 {
				return Entry{}, false
			}

			entriesLen := listLen - 4
			listStart := pos
			listEnd := listStart + entriesLen
			if listEnd > len(body) {
				return Entry{}, false
			}

			listEntries, ok := parseEntries(body, listStart, entriesLen, count, absStart)
			if !ok {
				return Entry{}, false
			}

			ent.Fields = append(ent.Fields, Field{Tag: tag, Type: typ, List: listEntries})
			pos = listEnd

		default:
			return Entry{}, false
		}
	}

	return ent, true
}
