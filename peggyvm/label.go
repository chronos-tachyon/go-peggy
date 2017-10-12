package peggyvm

import (
	"sort"
)

// Label represents metadata about a bytecode label. They are used while
// disassembling or debugging the bytecode.
type Label struct {
	Offset uint64
	Public bool
	Name   string
}

// Labels is an implementation of sort.Interface for *Label slices.
type Labels []*Label

var _ sort.Interface = (Labels)(nil)

func (x Labels) Len() int {
	return len(x)
}

func (x Labels) Less(i, j int) bool {
	a, b := x[i], x[j]
	if a.Offset != b.Offset {
		return a.Offset < b.Offset
	}
	if a.Public != b.Public {
		return a.Public
	}
	return a.Name < b.Name
}

func (x Labels) Swap(i, j int) {
	x[i], x[j] = x[j], x[i]
}
