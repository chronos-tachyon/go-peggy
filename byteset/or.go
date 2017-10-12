package byteset

// Or returns a Matcher that matches iff any of the given Matchers match.
//
// • Match performance: moderate (limited by inner matchers)
//
// • ForEach performance: moderate (limited by inner matchers)
//
// • Usefulness: situational
//
func Or(ms ...Matcher) Matcher {
	l := make([]Matcher, len(ms))
	copy(l, ms)
	return &mUnion{List: l}
}

type mUnion struct {
	List []Matcher
}

var _ Matcher = (*mUnion)(nil)

func (m *mUnion) Match(b byte) bool {
	for _, sub := range m.List {
		if sub.Match(b) {
			return true
		}
	}
	return false
}

func (m *mUnion) ForEach(f func(b byte)) {
	forEachUnion(m.List, f)
}

func (m *mUnion) Optimize() Matcher {
	if len(m.List) == 0 {
		return None()
	}
	if len(m.List) == 1 {
		return m.List[0].Optimize()
	}
	return asDense(m).Optimize()
}

func (m *mUnion) String() string {
	return genericString(m)
}
