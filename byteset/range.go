package byteset

import (
	"sort"
)

// Range represents a range of consecutive bytes.
//
// If Lo < Hi, then this Range represents the bytes Lo, Lo+1, ..., Hi-1, Hi.
//
// If Lo == Hi, then this Range represents the single byte Lo.
//
// If Lo > Hi, then this Range represents the null set.
//
type Range struct {
	Lo byte
	Hi byte
}

// Ranges returns a Matcher that matches any byte that falls in one of the
// given Range entries.
//
// • Match performance: moderate
//
// • ForEach performance: fast
//
// • Usefulness: broad
//
// This is usually the best choice if most of the bytes in your set are
// consecutive, and the number of such ranges is small.
//
func Ranges(rs ...Range) Matcher {
	return makeRange(rs)
}

type mRange struct {
	Ranges []Range
}

var _ Matcher = (*mRange)(nil)

func (m *mRange) Match(b byte) bool {
	i := sort.Search(len(m.Ranges), func(i int) bool {
		return m.Ranges[i].Hi >= b
	})
	if i >= len(m.Ranges) {
		return false
	}
	r := m.Ranges[i]
	return r.Lo <= b && b <= r.Hi
}

func (m *mRange) ForEach(f func(b byte)) {
	for _, r := range m.Ranges {
		for i := uint(r.Lo); i <= uint(r.Hi); i++ {
			f(byte(i))
		}
	}
}

func (m *mRange) Optimize() Matcher {
	if len(m.Ranges) == 0 {
		return None()
	}
	return m
}

func (m *mRange) String() string {
	return genericString(m)
}

func (m *mRange) asDense() Matcher {
	mm := &mDense{}
	for _, r := range m.Ranges {
		for x := uint(r.Lo); x <= uint(r.Hi); x++ {
			index, mask := denseIM(byte(x))
			mm.Set[index] |= mask
		}
	}
	return mm
}

func makeRange(rs []Range) *mRange {
	rs = coalesceRanges(rs)
	return &mRange{Ranges: rs}
}

func coalesceRanges(a []Range) []Range {
	// Because (*mRange).Match makes some assumptions for efficiency, we
	// have to guarantee that:
	//
	// - All Range entries have Lo <= Hi
	//
	// - There are no overlapping Range entries
	//
	// - The Range entries are sorted by Lo
	//   (Implied: m.Ranges[i-1].Hi <= m.Ranges[i].Lo)
	//
	// Since we're already doing all this work, we also coalesce
	// adjacent-but-non-overlapping ranges into a single range.

	// First: filter out entries where Lo > Hi, then sort by Lo
	b := make([]Range, 0, len(a))
	for _, r := range a {
		if r.Hi >= r.Lo {
			b = append(b, r)
		}
	}
	sort.Sort(rangeSlice(b))

	// Bail out early if there are only 0 or 1 entries.
	if len(b) < 2 {
		return b
	}

	// Second: merge adjacent and overlapping entries.
	//
	// Recall that entries have already been sorted by Lo ascending.
	//
	// We are left with four cases to handle:
	//
	// 1. Neither overlapping nor adjacent (keep as-is)
	//
	//   [a..b]
	//          [c..d]
	//   b < d && (b + 1) < c
	//   → [a..b],[c..d]
	//
	// 2. Adjacent (merge)
	//
	//   [a..b]
	//       [c..d]
	//   b < d && (b + 1) == c
	//   → [a..d]
	//
	// 3. Partially overlapping (merge)
	//
	//   [a..b]
	//      [c..d]
	//   b < d && (b + 1) > c
	//   → [a..d]
	//
	// 4. Fully overlapping (discard smaller)
	//
	//   [a......b]
	//     [c..d]
	//   b >= d
	//   → [a..b]
	//
	c := make([]Range, 0, len(b))
	var lastHi byte
	var have bool
	for _, r := range b {
		if have && lastHi >= r.Hi {
			// Case 4
			continue
		} else if have && lastHi >= r.Lo {
			// Cases 2 & 3
			c[len(c)-1].Hi = r.Hi
			lastHi = r.Hi
		} else {
			// Case 1
			c = append(c, r)
			lastHi = r.Hi
			have = true
		}
	}
	return c
}
