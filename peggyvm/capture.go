package peggyvm

import (
	"bytes"
	"fmt"
)

// CaptureMeta records metadata about one of a pattern's captures.
type CaptureMeta struct {
	// Name records the capture's name, if applicable.
	Name string

	// Repeat is true iff the compiled program can record multiple input
	// ranges for this capture.
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

// CapturePair is the start and end position of a single capture event.
type CapturePair struct {
	S uint64
	E uint64
}

// String provides a programmer-friendly debugging string for the CapturePair.
func (pair CapturePair) String() string {
	return fmt.Sprintf("(%d,%d)", pair.S, pair.E)
}

// Capture records all capture events that have occurred for a single index.
type Capture struct {
	// Exists is true iff at least one event is recorded.
	Exists bool

	// Solo is the most recent event.
	Solo CapturePair

	// Multi is a list of all events, oldest first.
	Multi []CapturePair
}

// String provides a programmer-friendly debugging string for the Capture.
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
