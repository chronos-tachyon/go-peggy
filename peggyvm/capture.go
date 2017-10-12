package peggyvm

import (
	"bytes"
	"fmt"
)

// CaptureMeta records metadata about one of a pattern's captures.
type CaptureMeta struct {
	Name   string
	Repeat bool
}

// Assignment records the start or end position of a capture.
type Assignment struct {
	// DP ("Data Pointer") is the index which is being recorded.
	//
	// - For a start assignment: this is the first captured byte
	//
	// - For an end assignment: this is one past the last captured byte
	//
	DP uint64

	// Index is the index of the capture being assigned to.
	Index uint64

	// IsEnd is true iff the end of the capture is being assigned, or false
	// iff the start of the capture is being assigned.
	IsEnd bool
}

type CapturePair struct {
	S uint64
	E uint64
}

func (pair CapturePair) String() string {
	return fmt.Sprintf("(%d,%d)", pair.S, pair.E)
}

type Capture struct {
	Exists bool
	Solo   CapturePair
	Multi  []CapturePair
}

func (c Capture) String() string {
	if !c.Exists {
		return "-"
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	buf.WriteString(c.Solo.String())
	if len(c.Multi) != 0 {
		buf.WriteByte(' ')
		buf.WriteByte('[')
		first := true
		for _, pair := range c.Multi {
			if !first {
				buf.WriteByte(' ')
			}
			buf.WriteString(pair.String())
			first = false
		}
		buf.WriteByte(']')
	}
	buf.WriteByte('}')
	return buf.String()
}
