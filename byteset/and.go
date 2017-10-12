package byteset

// And returns a Matcher that matches iff all of the given Matchers match.
//
// • Match performance: moderate (limited by inner matchers)
//
// • ForEach performance: moderate (limited by inner matchers)
//
// • Usefulness: situational
//
func And(ms ...Matcher) Matcher {
	l := make([]Matcher, len(ms))
	copy(l, ms)
	return &mIntersection{List: l}
}

type mIntersection struct {
	List []Matcher
}

var _ Matcher = (*mIntersection)(nil)

func (m *mIntersection) Match(b byte) bool {
	for _, sub := range m.List {
		if !sub.Match(b) {
			return false
		}
	}
	return true
}

func (m *mIntersection) ForEach(f func(b byte)) {
	if len(m.List) == 0 {
		forEachByte(0, 255, f)
		return
	}
	first := m.List[0]
	rest := m.List[1:]
	first.ForEach(func(b byte) {
		for _, sub := range rest {
			if !sub.Match(b) {
				return
			}
		}
		f(b)
	})
}

func (m *mIntersection) Optimize() Matcher {
	if len(m.List) == 0 {
		return All()
	}
	if len(m.List) == 1 {
		return m.List[0].Optimize()
	}
	return asDense(m).Optimize()
}

func (m *mIntersection) String() string {
	return genericString(m)
}
