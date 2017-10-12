package byteset

// Matcher is a predicate that returns true for certain bytes.
//
// For the sake of all that is good and holy, implementations of Matcher
// must *not* change their state on a call to Match.
//
type Matcher interface {
	// Match returns true iff byte b is in the set.
	Match(b byte) bool

	// ForEach calls f exactly once for each byte in the set. The arguments
	// for successive calls are guaranteed to be in ascending order.
	ForEach(f func(b byte))

	// Optimize returns a Matcher that matches the same set of bytes, but
	// possibly in a more efficient way. If no better implementation can be
	// found, returns this matcher.
	Optimize() Matcher

	// String returns a string representation of the set.
	String() string
}

type asDenser interface {
	asDense() Matcher
}

// Bytes appends each byte matched by m to out, then returns the updated slice.
func Bytes(m Matcher, out []byte) []byte {
	m.ForEach(func(b byte) { out = append(out, b) })
	return out
}

func asDense(m Matcher) Matcher {
	if md, ok := m.(*mDense); ok {
		return md
	}
	if mx, ok := m.(asDenser); ok {
		return mx.asDense()
	}
	mm := &mDense{}
	m.ForEach(func(b byte) {
		index, mask := denseIM(b)
		mm.Set[index] |= mask
	})
	return mm
}
