package tv4p

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// replacement represents a replacement operation in the tv4p file.
type replacement struct {
	blob  []byte // replacement bytes
	start int    // start offset of replacement
	end   int    // end offset of replacement
}

// PatchRoadTypes rewrites the Road Tool lists using the provided config.
//
// This includes:
// - road types list (0x88)
// - crossroad definitions list (0x89) if `cfg.CrossroadTypes` is present (non-nil)
// - crossroad links/metadata list (0x8A) if `cfg.CrossroadTypes` is present (non-nil)
func PatchRoadTypes(data []byte, cfg RoadConfig) ([]byte, error) {
	return PatchRoadTool(data, cfg, ScopeAll)
}

// PatchRoadTool rewrites parts of the Road Tool region according to scope.
//
// Scope behavior:
// - roads: patch only 0x88 (road types), preserve crossroads
// - crossroads: patch only 0x89 (crossroad defs) (and 0x8A only when raw link data is present), preserve road types
// - all: patch roads and crossroads
func PatchRoadTool(data []byte, cfg RoadConfig, scope Scope) ([]byte, error) {
	block, err := ParseRoadTypes(data)
	if err != nil {
		return nil, err
	}

	existingIDs := collectEntryIDs(data)
	repls := []replacement{}

	delta88 := 0
	delta89 := 0
	delta8A := 0

	if scope.IncludesRoads() {
		if len(cfg.Types) == 0 {
			return nil, errors.New("scope=roads requires road_types in config")
		}
		// TB appears to require road type entry IDs (0x88 entries, TypeID 0x12) to form
		// an arithmetic progression with a fixed stride (observed: 0x48). When we used
		// hash-derived IDs for generated configs, TB would rewrite them on save and could
		// misbehave during Create. We pre-assign missing road type IDs in a TB-like series.
		// If the config is effectively a round-trip update (same set of road types),
		// preserve IDs where possible; otherwise, assign a fresh, monotonic TB-like series.
		if len(cfg.Types) == len(block.Types) {
			inheritExistingRoadTypeIDs(&cfg, block.Types)
		}
		applySequentialRoadTypeIDs(&cfg, block.Types, existingIDs)

		roadTypeEntries, err := buildRoadTypesEntries(cfg, existingIDs)
		if err != nil {
			return nil, err
		}

		roadTypesField, err := fieldList(0x88, roadTypeEntries)
		if err != nil {
			return nil, err
		}
		new88ListLen := int(readU32(roadTypesField[3:]))
		old88EntriesLen := block.ListLen - 4
		new88EntriesLen := new88ListLen - 4
		delta88 = new88EntriesLen - old88EntriesLen

		// Replace 0x88 road types field.
		oldRoadTypesLen := 7 + block.ListLen
		repls = append(repls, replacement{
			start: block.Start,
			end:   block.Start + oldRoadTypesLen,
			blob:  roadTypesField,
		})
	}

	afterRoadTypes := block.Start + 7 + block.ListLen
	crDefs, _ := findTaggedListAfter(data, afterRoadTypes, 0x89, validateCrossroadDefs)
	crLinks, _ := findTaggedListAfter(data, afterRoadTypes, 0x8A, validateCrossroadLinks)

	// Only touch crossroads when config explicitly contains the key
	// (nil slice means "preserve whatever is in the file").
	if scope.IncludesCrossroads() && cfg.CrossroadTypes != nil {
		if !crDefs.Found || !crLinks.Found {
			return nil, errors.New("crossroad lists not found near Road Tool block")
		}

		// Crossroads-only patch: if road types are not provided in config, use the file's list
		// for index mapping and validation.
		if len(cfg.Types) == 0 {
			cfg.Types = block.Types
		}

		if err := ValidateCrossroads(cfg.CrossroadTypes, cfg.Types); err != nil {
			return nil, err
		}

		// TB Create fallback appears to use 0x89[roadTypeIndex] when variant selection is unreliable.
		// We reorder defs for generated configs and/or when explicit defaults are present.
		if shouldReorderCrossroads(cfg) {
			reorderCrossroadsByRoadTypeIndex(&cfg)
		}

		// If the config does not contain raw tv4p_link data, do NOT attempt to
		// synthesize/overwrite the 0x8A list. Despite being adjacent, 0x8A is not
		// a \"variant index\" table: it stores placed crossroad instances (position +
		// attached road parts) and is empty in many valid projects. Creating synthetic
		// entries does not fix TB's variant selection behavior.
		hasRawLink := false
		for i := range cfg.CrossroadTypes {
			if cfg.CrossroadTypes[i].TV4PLink != nil {
				hasRawLink = true
				break
			}
		}
		writeLinks := hasRawLink

		crossDefsField, crossLinksField, err := buildCrossroadFields(cfg, existingIDs, writeLinks)
		if err != nil {
			return nil, err
		}

		new89ListLen := int(readU32(crossDefsField[3:]))
		old89EntriesLen := crDefs.ListLen - 4
		new89EntriesLen := new89ListLen - 4
		delta89 = new89EntriesLen - old89EntriesLen
		delta8A = 0

		metaStart := crDefs.Start + crDefs.FieldLen
		metaEnd := crLinks.Start
		if metaStart < 0 || metaEnd < metaStart || metaEnd > len(data) {
			return nil, errors.New("invalid crossroads meta range")
		}

		repls = append(repls,
			replacement{start: crDefs.Start, end: crDefs.Start + crDefs.FieldLen, blob: crossDefsField},
		)

		if writeLinks {
			// Rewrite meta + 0x8A only when we are writing back real instance state from TB.
			new8AListLen := int(readU32(crossLinksField[3:]))
			old8AEntriesLen := crLinks.ListLen - 4
			new8AEntriesLen := new8AListLen - 4
			delta8A = new8AEntriesLen - old8AEntriesLen

			// There is a small metadata region between 0x89 and 0x8A in real files that contains
			// at least one u32 offset-like field (tag 0x3F/type 0x0D). Its value changes when
			// the 0x8A list payload size changes, so we must adjust it to keep the file consistent.
			metaBytes := append([]byte(nil), data[metaStart:metaEnd]...)
			if delta8A != 0 {
				if err := adjustU32FieldInSlice(metaBytes, 0x3F, 0x0D, delta8A); err != nil {
					return nil, err
				}
			}
			if err := setMetaLinkIDTail(metaBytes, crossLinksField); err != nil {
				return nil, err
			}

			repls = append(repls,
				replacement{start: metaStart, end: metaEnd, blob: metaBytes},
				replacement{start: crLinks.Start, end: crLinks.Start + crLinks.FieldLen, blob: crossLinksField},
			)
		}
	}

	// Apply replacements from end to start so earlier offsets remain valid.
	sortReplsDesc(repls)
	out := data
	totalDelta := 0
	for _, r := range repls {
		if r.start < 0 || r.end < r.start || r.end > len(out) {
			return nil, errors.New("invalid replacement range")
		}
		oldLen := r.end - r.start
		totalDelta += len(r.blob) - oldLen

		tmp := make([]byte, 0, len(out)-oldLen+len(r.blob))
		tmp = append(tmp, out[:r.start]...)
		tmp = append(tmp, r.blob...)
		tmp = append(tmp, out[r.end:]...)
		out = tmp
	}

	if totalDelta != 0 {
		// Observed behavior (from real files):
		// - tag 0x18/type 0x0D shifts by delta88 + delta89 + delta8A
		// - tag 0x3E/type 0x0D shifts by delta88 + delta89
		//
		// (delta8A does not affect 0x3E)
		if err := adjustOffsetsByTag(out, 0x18, 0x0D, delta88+delta89+delta8A); err != nil {
			return nil, err
		}
		if err := adjustOffsetsByTag(out, 0x3E, 0x0D, delta88+delta89); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// shouldReorderCrossroads determines if crossroads should be reordered based on defaults.
func shouldReorderCrossroads(cfg RoadConfig) bool {
	if len(cfg.CrossroadTypes) == 0 || len(cfg.Types) == 0 {
		return false
	}

	for i := range cfg.CrossroadTypes {
		if strings.TrimSpace(cfg.CrossroadTypes[i].Default) != "" {
			return true
		}
	}

	for i := range cfg.CrossroadTypes {
		// If we have any raw data, preserve stable order from TB/extract.
		if cfg.CrossroadTypes[i].TV4PDef != nil || cfg.CrossroadTypes[i].TV4PLink != nil {
			continue
		}
		// At least one generated entry => reorder to make defaults stable.
		return true
	}

	return false
}

// reorderCrossroadsByRoadTypeIndex reorders crossroads by road type index.
func reorderCrossroadsByRoadTypeIndex(cfg *RoadConfig) {
	if cfg == nil || len(cfg.Types) == 0 || len(cfg.CrossroadTypes) == 0 {
		return
	}

	shapeScore := func(cr CrossroadType) int {
		// Prefer T over X (arbitrary but stable).
		if strings.HasPrefix(cr.Name, "kr_t_") {
			return 2
		}
		if strings.HasPrefix(cr.Name, "kr_x_") {
			return 1
		}
		return 0
	}

	matchScore := func(cr CrossroadType, want string) int {
		// Explicit default always wins.
		if strings.TrimSpace(cr.Default) != "" && strings.ToLower(strings.TrimSpace(cr.Default)) == want {
			return 1000 + shapeScore(cr)
		}

		// Highest priority: AB (A/B) equals want.
		abA := strings.ToLower(cr.Connections.A)
		abB := strings.ToLower(cr.Connections.B)
		c := strings.ToLower(cr.Connections.C)
		d := strings.ToLower(cr.Connections.D)

		if abA == want && abB == want {
			return 100 + shapeScore(cr)
		}

		// Still pretty good: appears in A or B.
		if abA == want || abB == want {
			return 80 + shapeScore(cr)
		}

		// Fallback: appears in C/D.
		if c == want || d == want {
			return 60 + shapeScore(cr)
		}

		return -1
	}

	// Try to ensure that for a road type at index i, the crossroad definition at index i
	// has AB == that road type. This is done in-place by swapping entries.
	//
	// We only operate within the existing crossroad list length; if there are more road types
	// than crossroad defs, higher road types can't be aligned (we skip them).
	limit := len(cfg.Types)
	if limit > len(cfg.CrossroadTypes) {
		limit = len(cfg.CrossroadTypes)
	}

	// Track indices we successfully aligned so we don't break earlier placements.
	locked := make([]bool, len(cfg.CrossroadTypes))

	for rtIdx := 0; rtIdx < limit; rtIdx++ {
		wantAB := strings.ToLower(cfg.Types[rtIdx].Name)
		if wantAB == "" {
			continue
		}

		bestJ := -1
		bestScore := -1
		for j := 0; j < len(cfg.CrossroadTypes); j++ {
			if locked[j] && j != rtIdx {
				continue
			}
			cr := cfg.CrossroadTypes[j]
			s := matchScore(cr, wantAB)
			if s < 0 {
				continue
			}
			if s > bestScore {
				bestScore = s
				bestJ = j
			}
		}

		if bestJ == -1 || bestJ == rtIdx {
			continue
		}

		cfg.CrossroadTypes[rtIdx], cfg.CrossroadTypes[bestJ] = cfg.CrossroadTypes[bestJ], cfg.CrossroadTypes[rtIdx]
		locked[rtIdx] = true
	}
}

// inheritExistingRoadTypeIDs inherits existing road type IDs from the existing types.
func inheritExistingRoadTypeIDs(cfg *RoadConfig, existingTypes []RoadType) {
	byName := map[string]RoadType{}
	for _, rt := range existingTypes {
		if rt.Name == "" {
			continue
		}
		byName[strings.ToLower(rt.Name)] = rt
	}

	for i := range cfg.Types {
		rt := &cfg.Types[i]
		ex, ok := byName[strings.ToLower(rt.Name)]
		if !ok {
			continue
		}

		// Preserve road type ID if config doesn't specify one.
		if rt.ID == 0 && ex.ID != 0 {
			rt.ID = ex.ID
		}

		// Preserve part IDs where possible (by path, fallback to name+path).
		inheritPartListIDs(rt.StraightParts, ex.StraightParts)
		inheritPartListIDs(rt.CornerParts, ex.CornerParts)
		inheritPartListIDs(rt.TerminatorPart, ex.TerminatorPart)
	}
}

// inheritPartListIDs inherits existing part list IDs from the existing parts.
func inheritPartListIDs(dst []RoadPart, src []RoadPart) {
	byPath := map[string]uint32{}
	byKey := map[string]uint32{}

	for _, p := range src {
		if p.ID == 0 {
			continue
		}

		if p.Path != "" {
			byPath[strings.ToLower(p.Path)] = p.ID
		}
		key := strings.ToLower(p.Name) + "|" + strings.ToLower(p.Path)
		byKey[key] = p.ID
	}

	for i := range dst {
		if dst[i].ID != 0 {
			continue
		}

		if dst[i].Path != "" {
			if id, ok := byPath[strings.ToLower(dst[i].Path)]; ok {
				dst[i].ID = id
				continue
			}
		}

		key := strings.ToLower(dst[i].Name) + "|" + strings.ToLower(dst[i].Path)
		if id, ok := byKey[key]; ok {
			dst[i].ID = id
		}
	}
}

// applySequentialRoadTypeIDs applies sequential road type IDs to the configuration.
func applySequentialRoadTypeIDs(cfg *RoadConfig, existingTypes []RoadType, existingIDs map[uint32]struct{}) {
	const stride = uint32(0x48)

	// Determine the per-file remainder and current max ID from the existing file.
	var rem uint32
	var haveRem bool
	var maxID uint32
	for _, rt := range existingTypes {
		if rt.ID == 0 {
			continue
		}
		if !haveRem {
			rem = rt.ID % stride
			haveRem = true
		}
		if rt.ID > maxID {
			maxID = rt.ID
		}
	}

	// Also account for IDs explicitly provided in the incoming config.
	for i := range cfg.Types {
		if cfg.Types[i].ID == 0 {
			continue
		}
		if !haveRem {
			rem = cfg.Types[i].ID % stride
			haveRem = true
		}
		if cfg.Types[i].ID > maxID {
			maxID = cfg.Types[i].ID
		}
	}

	// If we still don't have a remainder (file had no road types), pick a stable one.
	// We avoid 0 to reduce chance of special-casing.
	if !haveRem {
		rem = 0x0C
	}

	// Allocate new IDs starting after the max ID, keeping the remainder.
	next := maxID + stride
	off := next % stride
	if off != rem {
		if off < rem {
			next += rem - off
		} else {
			next += (stride - off) + rem
		}
	}

	for i := range cfg.Types {
		if cfg.Types[i].ID != 0 {
			continue
		}

		// Find the next free ID in the arithmetic progression.
		for next == 0 || next%stride != rem {
			next += stride
		}
		for {
			if _, used := existingIDs[next]; used || next == 0 {
				next += stride
				continue
			}
			cfg.Types[i].ID = next
			existingIDs[next] = struct{}{}
			next += stride
			break
		}
	}
}

// adjustU32FieldInSlice adjusts a u32 field in a slice by a given delta.
func adjustU32FieldInSlice(b []byte, tag byte, typ byte, delta int) error {
	if delta == 0 {
		return nil
	}

	pat := []byte{tag, 0x00, typ}
	pos := bytes.Index(b, pat)
	if pos < 0 || pos+7 > len(b) {
		return errors.New("u32 field not found for adjustment")
	}

	// Make sure it's unique within this slice (sanity).
	if bytes.Contains(b[pos+1:], pat) {
		return errors.New("u32 field ambiguous for adjustment")
	}

	cur := int(readU32(b[pos+3:]))
	cur += delta
	if cur < 0 {
		cur = 0
	}

	return writeU32FromInt(b[pos+3:], cur)
}

// setMetaLinkIDTail sets the link ID tail in the meta.
func setMetaLinkIDTail(meta []byte, crossLinksField []byte) error {
	// Find 19 00 20 within meta.
	pat := []byte{0x19, 0x00, 0x20}
	pos := bytes.Index(meta, pat)
	if pos < 0 || pos+6 > len(meta) {
		return errors.New("crossroads meta 0x19/0x20 field not found")
	}
	// Ensure it's unique within this slice.
	if bytes.Contains(meta[pos+1:], pat) {
		return errors.New("crossroads meta 0x19/0x20 field ambiguous")
	}

	// crossLinksField is: 8A 00 0C <u32 listLen> <u32 count> <entries...>
	if len(crossLinksField) < 11 {
		return errors.New("invalid 0x8A field bytes")
	}
	count := int(readU32(crossLinksField[7:]))
	if count <= 0 {
		// No entries: keep whatever is already in meta (matches empty-list files).
		return nil
	}

	entriesStart := 11
	if len(crossLinksField) < entriesStart+7+6 {
		return errors.New("invalid 0x8A entries payload")
	}

	// First entry is 06 00 0D <u32 bodyLen> <u16 type> <u32 id> ...
	if crossLinksField[entriesStart] != 0x06 || crossLinksField[entriesStart+1] != 0x00 || crossLinksField[entriesStart+2] != 0x0D {
		return errors.New("invalid 0x8A first entry header")
	}
	bodyLen := int(readU32(crossLinksField[entriesStart+3:]))
	bodyStart := entriesStart + 7
	if bodyLen < 6 || len(crossLinksField) < bodyStart+6 {
		return errors.New("invalid 0x8A first entry body")
	}
	// typeID := readU16(crossLinksField[bodyStart:]) // not used, expected 0x1A
	idBytes := crossLinksField[bodyStart+2 : bodyStart+6]
	// meta expects the upper 3 bytes of the entry ID (skip the low byte).
	meta[pos+3] = idBytes[1]
	meta[pos+4] = idBytes[2]
	meta[pos+5] = idBytes[3]
	return nil
}

// buildRoadTypesEntries builds the road types list entries from the configuration.
func buildRoadTypesEntries(cfg RoadConfig, existingIDs map[uint32]struct{}) ([][]byte, error) {
	alloc := newIDAllocator(cfg, existingIDs)
	var entries [][]byte
	for _, rt := range cfg.Types {
		entry, err := buildRoadTypeEntry(rt, alloc)
		if err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	return entries, nil
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
	next uint32
}

// newIDAllocator creates a new ID allocator.
func newIDAllocator(cfg RoadConfig, existing map[uint32]struct{}) *idAllocator {
	used := map[uint32]struct{}{}
	var maxUsed uint32
	for id := range existing {
		if id != 0 {
			used[id] = struct{}{}
			if id > maxUsed {
				maxUsed = id
			}
		}
	}

	for _, rt := range cfg.Types {
		if rt.ID != 0 {
			used[rt.ID] = struct{}{}
			if rt.ID > maxUsed {
				maxUsed = rt.ID
			}
		}
		for _, p := range rt.StraightParts {
			if p.ID != 0 {
				used[p.ID] = struct{}{}
				if p.ID > maxUsed {
					maxUsed = p.ID
				}
			}
		}
		for _, p := range rt.CornerParts {
			if p.ID != 0 {
				used[p.ID] = struct{}{}
				if p.ID > maxUsed {
					maxUsed = p.ID
				}
			}
		}
		for _, p := range rt.TerminatorPart {
			if p.ID != 0 {
				used[p.ID] = struct{}{}
				if p.ID > maxUsed {
					maxUsed = p.ID
				}
			}
		}
	}

	// Terrain Builder appears to allocate IDs sequentially in blocks.
	// For generated configs, deterministic hash-based IDs can confuse TB's internal lookup.
	// We therefore allocate new IDs sequentially from the current file's ID space.
	next := maxUsed + 1
	// Keep basic alignment (most observed IDs are at least 4-byte aligned).
	if rem := next % 4; rem != 0 {
		next += 4 - rem
	}

	return &idAllocator{used: used, next: next}
}

// allocNext allocates a next ID.
func (a *idAllocator) allocNext() uint32 {
	// Keep trying until we find an unused non-zero ID.
	for {
		id := a.next
		// Increment first so we don't get stuck on a used value.
		a.next++
		if a.next == 0 {
			// wrap-around (extremely unlikely); skip 0.
			a.next = 1
		}

		if id == 0 {
			continue
		}
		if _, ok := a.used[id]; ok {
			continue
		}
		a.used[id] = struct{}{}
		return id
	}
}

// useOrDeterministic allocates an ID or uses a deterministic ID.
func (a *idAllocator) useOrDeterministic(id uint32, seed string) uint32 {
	if id != 0 {
		a.used[id] = struct{}{}
		return id
	}

	// NOTE: `seed` kept for signature stability and potential future debugging.
	_ = seed
	return a.allocNext()
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
		// Observed in real tv4p files: even when the custom flag is false,
		// the stored RGBA is 00 00 00 FF (alpha stays 0xFF).
		return []byte{tag, 0x00, 0x08, 0x00, 0x00, 0x00, 0xFF}
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

// sortReplsDesc sorts replacements by start index in descending order.
func sortReplsDesc(repls []replacement) {
	for i := 0; i < len(repls); i++ {
		for j := i + 1; j < len(repls); j++ {
			if repls[j].start > repls[i].start {
				repls[i], repls[j] = repls[j], repls[i]
			}
		}
	}
}

// buildCrossroadFields builds the crossroad fields from the configuration.
func buildCrossroadFields(cfg RoadConfig, existingIDs map[uint32]struct{}, includeLinks bool) ([]byte, []byte, error) {
	alloc := newIDAllocator(cfg, existingIDs)

	nameToIdx := map[string]uint32{}
	for idx := uint32(0); uint64(idx) < uint64(len(cfg.Types)); idx++ {
		i := int(idx)
		nameToIdx[cfg.Types[i].Name] = idx
	}

	// Allocate crossroad definition IDs in a TB-like pattern.
	// In TB-made files crossroad def entry IDs (TypeID 0x17) often increment by 0x178.
	// Random-looking IDs appear to confuse TB when selecting a specific crossroad variant.
	defIDs := allocateCrossroadDefIDs(cfg.CrossroadTypes, alloc)

	// Build 0x89 entries
	var defEntries [][]byte
	for i, cr := range cfg.CrossroadTypes {
		e, err := buildCrossroadDefEntry(cr, alloc, nameToIdx, defIDs[i])
		if err != nil {
			return nil, nil, err
		}
		defEntries = append(defEntries, e)
	}

	defField, err := fieldList(0x89, defEntries)
	if err != nil {
		return nil, nil, err
	}

	if !includeLinks {
		// Caller will preserve existing meta + 0x8A.
		return defField, nil, nil
	}

	// Build 0x8A entries.
	//
	// Important: in real TB files `0x8A` does NOT scale with the number of crossroad definitions.
	// Example: `utesplus_cross2.tv4p` has 2 entries in `0x89`, but still only 1 entry in `0x8A`.
	// This list appears to be editor state / metadata, not per-crossroad definition.
	var linkEntries [][]byte
	{
		// If we have any raw link entry from extract, write back one verbatim (TB state).
		var picked *CrossroadType
		for i := range cfg.CrossroadTypes {
			if cfg.CrossroadTypes[i].TV4PLink != nil && cfg.CrossroadTypes[i].TV4PLink.Type == 0x1A {
				picked = &cfg.CrossroadTypes[i]
				break
			}
		}
		if picked != nil {
			e, err := buildCrossroadLinkEntry(*picked, alloc, cfg.Types)
			if err != nil {
				return nil, nil, err
			}
			linkEntries = append(linkEntries, e)
		}
	}

	linkField, err := fieldList(0x8A, linkEntries)
	if err != nil {
		return nil, nil, err
	}

	return defField, linkField, nil
}

// buildCrossroadDefEntry builds the crossroad definition entry from the configuration.
func buildCrossroadDefEntry(cr CrossroadType, alloc *idAllocator, nameToIdx map[string]uint32, forcedID uint32) ([]byte, error) {
	seed := "crdef|" + strings.ToLower(cr.Name) + "|" + strings.ToLower(cr.Model)

	// If we have a raw entry from extract, write it back verbatim.
	// This is the safest option and enables true round-trip.
	if cr.TV4PDef != nil && cr.TV4PDef.Type == 0x17 {
		return rawEntryToBytes(*cr.TV4PDef, alloc, seed)
	}

	// Shape enum (observed): 2 for T, 3 for X.
	shapeU32 := uint32(2)
	if strings.HasPrefix(cr.Name, "kr_x_") {
		shapeU32 = 3
	}

	a := uint32(0xFFFFFFFF)
	b := uint32(0xFFFFFFFF)
	c := uint32(0xFFFFFFFF)
	okA, okB, okC := true, true, true
	if cr.Connections.A != "" {
		a, okA = nameToIdx[cr.Connections.A]
	}
	if cr.Connections.B != "" {
		b, okB = nameToIdx[cr.Connections.B]
	}
	if cr.Connections.C != "" {
		c, okC = nameToIdx[cr.Connections.C]
	}
	d := uint32(0xFFFFFFFF)
	okD := true
	if cr.Connections.D != "" {
		dd, ok := nameToIdx[cr.Connections.D]
		okD = ok
		d = dd
	}

	if cr.Connections.A != "" && !okA {
		return nil, fmt.Errorf("crossroad %q: unknown road type for A: %q", cr.Name, cr.Connections.A)
	}
	if cr.Connections.B != "" && !okB {
		return nil, fmt.Errorf("crossroad %q: unknown road type for B: %q", cr.Name, cr.Connections.B)
	}
	if cr.Connections.C != "" && !okC {
		return nil, fmt.Errorf("crossroad %q: unknown road type for C: %q", cr.Name, cr.Connections.C)
	}
	if cr.Connections.D != "" && !okD {
		return nil, fmt.Errorf("crossroad %q: unknown road type for D: %q", cr.Name, cr.Connections.D)
	}

	raw := EntryRaw{
		Type: 0x17,
		ID:   forcedID,
		Fields: []FieldRaw{
			{Tag: 0x33, Type: 0x0B, Raw: hex.EncodeToString([]byte(cr.Name))},
			{Tag: 0x7C, Type: 0x0B, Raw: hex.EncodeToString([]byte(cr.Model))},
			{Tag: 0x7F, Type: 0x05, Raw: u32Hex(shapeU32)},
			// 0x71 controls whether the UI color is \"custom\".
			// Observed in multiple TB-made files: when color is \"standard\", 0x71=0
			// and 0x73 is a fixed sentinel 00 00 FF 00 (not a real RGBA color).
			{Tag: 0x71, Type: 0x09, Raw: func() string {
				if cr.ColorCustom {
					return "01"
				}
				return "00"
			}()},
			{Tag: 0x73, Type: 0x08, Raw: func() string {
				if cr.ColorCustom {
					return rgbaHex(cr.Color)
				}
				// TB \"standard\" sentinel:
				return "0000ff00"
			}()},
			{Tag: 0x75, Type: 0x09, Raw: "00"},
			{Tag: 0x80, Type: 0x14, Raw: "0000000000000000"},
			{Tag: 0x81, Type: 0x14, Raw: "0000000000000000"},
			{Tag: 0x82, Type: 0x14, Raw: "0000000000000000"},
			{Tag: 0x83, Type: 0x09, Raw: "00"},
			{Tag: 0x84, Type: 0x05, Raw: u32Hex(a)},
			{Tag: 0x85, Type: 0x05, Raw: u32Hex(b)},
			{Tag: 0x86, Type: 0x05, Raw: u32Hex(c)},
			{Tag: 0x87, Type: 0x05, Raw: u32Hex(d)},
		},
	}

	return rawEntryToBytes(raw, alloc, seed)
}

// allocateCrossroadDefIDs allocates crossroad definition IDs.
func allocateCrossroadDefIDs(crossroads []CrossroadType, alloc *idAllocator) []uint32 {
	// Keep stable mapping by position.
	out := make([]uint32, len(crossroads))
	if len(crossroads) == 0 {
		return out
	}

	const stride = 0x178

	// If any crossroad already has a raw ID (from extract), we keep zero here (unused).
	// For generated ones, we allocate sequential IDs with a fixed stride and avoid collisions.
	need := 0
	for i := range crossroads {
		if crossroads[i].TV4PDef == nil {
			need++
		}
	}
	if need == 0 {
		return out
	}

	// Choose a base in the current file ID space (not a hash-derived value).
	// We try a few candidates starting at alloc.next (aligned to 4 bytes).
	base := alloc.next
	if rem := base % 4; rem != 0 {
		base += 4 - rem
	}

	for attempt := uint32(0); attempt < 4096; attempt++ {
		ok := true
		cur := base + attempt

		for idx := uint32(0); uint64(idx) < uint64(len(crossroads)); idx++ {
			i := int(idx)
			if crossroads[i].TV4PDef != nil {
				continue
			}

			// Avoid gosec G115: compute in uint64 and bounds-check.
			id64 := uint64(cur) + uint64(idx)*uint64(stride)
			if id64 > 0xFFFFFFFF {
				ok = false
				break
			}
			id := uint32(id64)
			if id == 0 {
				ok = false
				break
			}

			if _, exists := alloc.used[id]; exists {
				ok = false
				break
			}

			out[i] = id
		}

		if ok {
			for _, id := range out {
				if id != 0 {
					alloc.used[id] = struct{}{}
					if id >= alloc.next {
						alloc.next = id + 1
					}
				}
			}

			// Keep alignment for subsequent sequential allocations.
			if rem := alloc.next % 4; rem != 0 {
				alloc.next += 4 - rem
			}
			return out
		}
	}

	// Extremely defensive fallback: just allocate individually.
	for i := range crossroads {
		if crossroads[i].TV4PDef != nil {
			continue
		}
		out[i] = alloc.allocNext()
	}

	return out
}

func buildCrossroadLinkEntry(cr CrossroadType, alloc *idAllocator, roadTypes []RoadType) ([]byte, error) {
	seed := "crlink|" + strings.ToLower(cr.Name) + "|" + strings.ToLower(cr.Model)

	// If we have a raw link entry from extract, write it back verbatim.
	// This is required for stable behavior in Terrain Builder; the semantics of
	// the nested lists and vector fields are not fully reverse engineered yet.
	if cr.TV4PLink != nil && cr.TV4PLink.Type == 0x1A {
		return rawEntryToBytes(*cr.TV4PLink, alloc, seed)
	}

	shapeU32 := uint32(2)
	if strings.HasPrefix(cr.Name, "kr_x_") {
		shapeU32 = 3
	}

	// Minimal template (best-effort) when raw is unavailable.
	// We mimic the observed field ordering from real files.
	raw := EntryRaw{
		Type: 0x1A,
		ID:   alloc.useOrDeterministic(0, seed),
		Fields: []FieldRaw{
			{Tag: 0x8C, Type: 0x14, Raw: "0000000000000000"},
			{Tag: 0x8D, Type: 0x14, Raw: "0000000000000000"},
			{Tag: 0x8E, Type: 0x15, Raw: vec2f64Hex(0, 0)},
			{Tag: 0x8F, Type: 0x05, Raw: u32Hex(0)},
			{Tag: 0x90, Type: 0x05, Raw: u32Hex(shapeU32)},
			{Tag: 0x91, Type: 0x0B, Raw: hex.EncodeToString([]byte(cr.Model))},
			{Tag: 0x92, Type: 0x0C, List: nil},
			{Tag: 0x93, Type: 0x0C, List: nil},
			{Tag: 0x94, Type: 0x0C, List: nil},
			{Tag: 0x95, Type: 0x0C, List: nil},
		},
	}

	// Best-effort: put one reference entry into 0x92 list using AB type.
	sideLists, err := buildCrossroadSideLists(cr, alloc, roadTypes)
	if err != nil {
		return nil, err
	}
	if lst, ok := sideLists[0x92]; ok && len(lst) > 0 {
		raw.Fields[6].List = lst
	}

	return rawEntryToBytes(raw, alloc, seed)
}

func buildCrossroadSideLists(cr CrossroadType, alloc *idAllocator, roadTypes []RoadType) (map[uint8][]EntryRaw, error) {
	// Map: 0x92=A, 0x93=B, 0x94=C, 0x95=D
	out := map[uint8][]EntryRaw{
		0x92: nil,
		0x93: nil,
		0x94: nil,
		0x95: nil,
	}

	byName := map[string]RoadType{}
	for _, rt := range roadTypes {
		byName[rt.Name] = rt
	}

	refFor := func(rtName string) (string, error) {
		if rtName == "" {
			return "", nil
		}
		rt, ok := byName[rtName]
		if !ok {
			return "", fmt.Errorf("crossroad %q: unknown road type %q", cr.Name, rtName)
		}

		// Prefer *_12 straight part.
		want := rtName + "_12"
		for _, p := range rt.StraightParts {
			if p.Name == want {
				return p.Path, nil
			}
		}

		// Fallback: lexicographically smallest straight part name.
		bestName := ""
		bestPath := ""
		for _, p := range rt.StraightParts {
			if bestName == "" || p.Name < bestName {
				bestName = p.Name
				bestPath = p.Path
			}
		}

		return bestPath, nil
	}

	mkRefEntry := func(side string, path string) EntryRaw {
		raw := EntryRaw{Type: 0x1B}
		raw.ID = alloc.useOrDeterministic(0, "crref|"+strings.ToLower(cr.Model)+"|"+side+"|"+strings.ToLower(path))
		raw.Fields = []FieldRaw{
			{Tag: 0x7F, Type: 0x05, Raw: u32Hex(3)},
			{Tag: 0x6C, Type: 0x05, Raw: u32Hex(1)},
			{Tag: 0x33, Type: 0x0B, Raw: hex.EncodeToString([]byte(path))},
		}
		return raw
	}

	aPath, err := refFor(cr.Connections.A)
	if err != nil {
		return nil, err
	}
	bPath, err := refFor(cr.Connections.B)
	if err != nil {
		return nil, err
	}
	cPath, err := refFor(cr.Connections.C)
	if err != nil {
		return nil, err
	}
	dPath, err := refFor(cr.Connections.D)
	if err != nil {
		return nil, err
	}

	if aPath != "" {
		e := mkRefEntry("A", aPath)
		out[0x92] = []EntryRaw{e}
	}
	if bPath != "" {
		e := mkRefEntry("B", bPath)
		out[0x93] = []EntryRaw{e}
	}
	if cPath != "" {
		e := mkRefEntry("C", cPath)
		out[0x94] = []EntryRaw{e}
	}
	if dPath != "" {
		e := mkRefEntry("D", dPath)
		out[0x95] = []EntryRaw{e}
	}

	return out, nil
}

// rawEntryToBytes converts a raw entry to bytes.
func rawEntryToBytes(e EntryRaw, alloc *idAllocator, seed string) ([]byte, error) {
	if e.Type == 0 {
		return nil, errors.New("raw entry missing type")
	}

	id := e.ID
	if id == 0 {
		id = alloc.useOrDeterministic(0, seed)
	}

	var fields [][]byte
	for _, f := range e.Fields {
		b, err := rawFieldToBytes(f, alloc, seed)
		if err != nil {
			return nil, err
		}
		fields = append(fields, b)
	}

	return buildEntry(e.Type, id, fields)
}

// rawFieldToBytes converts a raw field to bytes.
func rawFieldToBytes(f FieldRaw, alloc *idAllocator, seed string) ([]byte, error) {
	tag := f.Tag
	typ := f.Type

	switch typ {
	case 0x0C:
		// Nested list.
		var entries [][]byte
		for _, re := range f.List {
			rseed := seed + "|sub|" + strconv.Itoa(int(re.Type)) + "|" + strconv.Itoa(int(re.ID))
			b, err := rawEntryToBytes(re, alloc, rseed)
			if err != nil {
				return nil, err
			}
			entries = append(entries, b)
		}

		return fieldList(tag, entries)

	case 0x0B:
		raw, err := decodeHex(f.Raw)
		if err != nil {
			return nil, err
		}

		out := make([]byte, 0, len(raw)+5)
		out = append(out, tag, 0x00, typ)
		tmp := make([]byte, 2)
		if err := writeU16FromInt(tmp, len(raw)); err != nil {
			return nil, err
		}

		out = append(out, tmp...)
		out = append(out, raw...)

		return out, nil

	case 0x09, 0x08, 0x14, 0x05, 0x0D, 0x15, 0x20:
		raw, err := decodeHex(f.Raw)
		if err != nil {
			return nil, err
		}

		out := make([]byte, 0, len(raw)+3)
		out = append(out, tag, 0x00, typ)
		out = append(out, raw...)

		return out, nil

	default:
		return nil, fmt.Errorf("unsupported raw field type: 0x%02X", typ)
	}
}

// decodeHex decodes a hex string to a byte slice.
func decodeHex(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// u32Hex converts a uint32 to a hex string.
func u32Hex(v uint32) string {
	var b [4]byte
	writeU32(b[:], v)
	return hex.EncodeToString(b[:])
}

// rgbaHex converts a color to a hex string.
func rgbaHex(c Color) string {
	return hex.EncodeToString([]byte{c.R, c.G, c.B, c.A})
}

// vec2f64Hex converts a 2D vector to a hex string.
func vec2f64Hex(x float64, y float64) string {
	// type 0x15 is: byte count + N*8 bytes
	var b [1 + 16]byte
	b[0] = 2
	binary.LittleEndian.PutUint64(b[1:], math.Float64bits(x))
	binary.LittleEndian.PutUint64(b[9:], math.Float64bits(y))

	return hex.EncodeToString(b[:])
}
