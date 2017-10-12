package byteset

// Exactly returns a Matcher that matches one specific byte.
//
// • Match performance: fast
//
// • ForEach performance: fast
//
// • Usefulness: situational
//
// This is the best choice if you want to match exactly one byte.
//
func Exactly(b byte) Matcher {
	return &mExact{Byte: b}
}

type mExact struct{ Byte byte }

var _ Matcher = (*mExact)(nil)

func (m *mExact) Match(b byte) bool {
	return b == m.Byte
}

func (m *mExact) ForEach(f func(b byte)) {
	f(m.Byte)
}

func (m *mExact) Optimize() Matcher {
	return m
}

func (m *mExact) String() string {
	return genericString(m)
}

func (m *mExact) asDense() Matcher {
	index, mask := denseIM(m.Byte)
	mm := &mDense{}
	mm.Set[index] = mask
	return mm
}
