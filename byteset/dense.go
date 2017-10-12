package byteset

// DenseSet returns a Matcher that matches any of the given bytes.
//
// • Match performance: fast
//
// • ForEach performance: slow
//
// • Usefulness: broad
//
// This is usually the best choice if your set doesn't have a clear pattern.
//
func DenseSet(given ...byte) Matcher {
	m := &mDense{}
	for _, b := range given {
		index, mask := denseIM(b)
		if (m.Set[index] & mask) == 0 {
			m.Set[index] |= mask
		}
	}
	return m
}

type mDense struct {
	Set [8]uint32
}

var _ Matcher = (*mDense)(nil)

func (m *mDense) Match(b byte) bool {
	index, mask := denseIM(b)
	return (m.Set[index] & mask) == mask
}

func (m *mDense) ForEach(f func(b byte)) {
	for i := uint(0); i < 8; i++ {
		for j := uint(0); j < 32; j++ {
			mask := uint32(1) << j
			if (m.Set[i] & mask) == mask {
				b := byte(i << 5) | byte(j)
				f(b)
			}
		}
	}
}

func (m *mDense) Optimize() Matcher {
	var n uint
	m.ForEach(func(_ byte) { n += 1 })

	switch n {
	case 0:
		return None()
	case 256:
		return All()
	case 1:
		var bb byte
		m.ForEach(func(b byte) {
			bb = b
		})
		return Exactly(bb)
	}
	return m
}

func (m *mDense) String() string {
	return genericString(m)
}

func denseIM(b byte) (index uint, mask uint32) {
	i := uint((b & 0xe0) >> 5)
	j := uint(b & 0x1f)
	mask = uint32(1) << j
	return i, mask
}
