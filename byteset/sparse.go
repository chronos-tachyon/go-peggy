package byteset

import (
	"sort"
)

// SparseSet returns a Matcher that matches any of the given bytes.
//
// • Match performance: fast
//
// • ForEach performance: moderate
//
// • Usefulness: broad
//
// This is usually the best choice if your set is small-ish and is mostly made
// of non-consecutive bytes.
//
func SparseSet(given ...byte) Matcher {
	set := make(map[byte]struct{}, len(given))
	for _, b := range given {
		set[b] = struct{}{}
	}
	return &mSparse{Set: set}
}

type mSparse struct {
	Set map[byte]struct{}
}

var _ Matcher = (*mSparse)(nil)

func (m *mSparse) Match(b byte) bool {
	_, found := m.Set[b]
	return found
}

func (m *mSparse) ForEach(f func(b byte)) {
	sorted := make([]byte, 0, len(m.Set))
	for b := range m.Set {
		sorted = append(sorted, b)
	}
	sort.Sort(byteSlice(sorted))
	for _, b := range sorted {
		f(b)
	}
}

func (m *mSparse) Optimize() Matcher {
	if len(m.Set) == 0 {
		return None()
	}
	if len(m.Set) == 1 {
		for b := range m.Set {
			return Exactly(b)
		}
	}
	return m
}

func (m *mSparse) String() string {
	return genericString(m)
}

func (m *mSparse) asDense() Matcher {
	mm := &mDense{}
	for b := range m.Set {
		index, mask := denseIM(b)
		mm.Set[index] |= mask
	}
	return mm
}
