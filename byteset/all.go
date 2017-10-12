package byteset

// All returns a Matcher that matches all possible bytes.
//
// • Match performance: fast
//
// • ForEach performance: slow
//
// • Usefulness: situational
//
func All() Matcher { return singletonAll }

type mAll struct{}

var _ Matcher = (*mAll)(nil)
var singletonAll = &mAll{}

func (m *mAll) Match(b byte) bool      { return true }
func (m *mAll) ForEach(f func(b byte)) { genericForEach(m, f) }
func (m *mAll) Optimize() Matcher      { return singletonAll }
func (m *mAll) String() string         { return "." }
