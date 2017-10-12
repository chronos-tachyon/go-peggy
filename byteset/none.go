package byteset

// None returns a Matcher that never matches any bytes.
//
// • Match performance: fast
//
// • ForEach performance: fast
//
// • Usefulness: situational
//
func None() Matcher { return singletonNone }

type mNone struct{}

var _ Matcher = (*mNone)(nil)
var singletonNone = &mNone{}

func (m *mNone) Match(b byte) bool      { return false }
func (m *mNone) ForEach(f func(b byte)) {}
func (m *mNone) Optimize() Matcher      { return singletonNone }
func (m *mNone) String() string         { return "!." }
