// Package tv4p provides parsing and patching of Terrain Builder tv4p road types.
package tv4p

// RoadConfig is a serialized config of road types from Terrain Builder.
type RoadConfig struct {
	Types []RoadType `json:"road_types"`
}

// RoadTypesBlock represents the raw road types list block inside a tv4p file.
type RoadTypesBlock struct {
	Entries      []Entry    // raw entries for road types
	Types        []RoadType // decoded road types for UI fields
	Start        int        // offset of the road types list header (tag 0x88/0x0C)
	ListLen      int        // byte length of the list payload (+4 for count)
	EntriesStart int        // offset of the first entry in the list payload
	EntriesLen   int        // byte length of entries payload (listLen - 4)
	Count        uint32     // number of road types in this block
}

// Entry is a raw tv4p entry (type/id + fields).
type Entry struct {
	Fields   []Field // raw fields (strings, colors, lists, flags)
	Offset   int     // absolute offset of this entry in the tv4p file
	IDOffset int     // absolute offset of the 4-byte ID inside this entry
	ID       uint32  // entry ID used by Terrain Builder
	TypeID   uint16  // entry type (e.g. 0x12 road type, 0x13 straight, 0x14 corner, 0x16 terminator)
}

// Field is a raw tv4p field inside an entry.
type Field struct {
	Raw  []byte  // raw payload for non-list fields
	List []Entry // nested entries for list fields
	Tag  byte    // field tag (e.g. 0x33 name, 0x7C path)
	Type byte    // field type (0x0B string, 0x08 color, 0x0C list, etc.)
}

// RoadType is a road type as shown in Terrain Builder Road Types window.
type RoadType struct {
	Name           string     `json:"name"`                // road type name (e.g. asf1)
	StraightParts  []RoadPart `json:"starting_parts"`      // Starting Parts tab
	CornerParts    []RoadPart `json:"corner_parts"`        // Corner Parts tab
	TerminatorPart []RoadPart `json:"terminator_parts"`    // Terminator Parts tab
	ID             uint32     `json:"id,omitempty"`        // internal ID for this road type
	Type           uint16     `json:"type"`                // entry type for road type (usually 0x12)
	KeyColor       Color      `json:"key_parts_color"`     // Key Parts Color (UI)
	NormalColor    Color      `json:"normal_parts_color"`  // Normal Parts Color (UI)
	KeyCustom      bool       `json:"key_parts_custom"`    // Key Parts Color is custom (not default)
	NormalCustom   bool       `json:"normal_parts_custom"` // Normal Parts Color is custom (not default)
}

// Color is an RGBA color used for road parts UI.
type Color struct {
	R byte `json:"r"` // red component
	G byte `json:"g"` // green component
	B byte `json:"b"` // blue component
	A byte `json:"a"` // alpha component
}

// RoadPart is an entry from one of the three parts lists.
type RoadPart struct {
	Name string `json:"name"`         // part name (e.g. asf2_7 100)
	Path string `json:"object_file"`  // Object File path from UI (p3d)
	ID   uint32 `json:"id,omitempty"` // internal ID for this part
	Type uint16 `json:"type"`         // entry type (0x13 straight, 0x14 corner, 0x16 terminator)
}

// roadTypesMeta represents the internal offsets of the road types list.
type roadTypesMeta struct {
	Start        int // offset of the road types list header
	ListLen      int // list length as stored in tv4p (includes 4 bytes count)
	EntriesStart int // offset of the entries payload
	EntriesLen   int // length of entries payload
}
