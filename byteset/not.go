package byteset

// Not returns a Matcher that inverts the given Matcher.
//
// • Match performance: fast (limited by inner matcher)
//
// • ForEach performance: slow
//
// • Usefulness: situational
//
func Not(m Matcher) Matcher {
	return &mNegation{Inner: m}
}

type mNegation struct {
	Inner Matcher
}

var _ Matcher = (*mNegation)(nil)

func (m *mNegation) Match(b byte) bool {
	return !m.Inner.Match(b)
}

func (m *mNegation) ForEach(f func(b byte)) {
	genericForEach(m, f)
}

func (m *mNegation) Optimize() Matcher {
	m.Inner = m.Inner.Optimize()
	switch sub := m.Inner.(type) {
	case *mAll:
		return None()
	case *mNone:
		return All()
	case *mNegation:
		return sub.Inner
	case *mDense:
		mm := &mDense{}
		for i := uint(0); i < 8; i++ {
			mm.Set[i] = ^sub.Set[i]
		}
		return mm
	default:
		return m
	}
}

func (m *mNegation) String() string {
	return "!" + m.Inner.String()
}
