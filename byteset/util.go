package byteset

import (
	"bytes"
	"fmt"
	"sort"
)

type byteSlice []byte

var _ sort.Interface = (byteSlice)(nil)

func (x byteSlice) Len() int           { return len(x) }
func (x byteSlice) Less(i, j int) bool { return x[i] < x[j] }
func (x byteSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

type byteSliceReverse []byte

var _ sort.Interface = (byteSliceReverse)(nil)

func (x byteSliceReverse) Len() int           { return len(x) }
func (x byteSliceReverse) Less(i, j int) bool { return x[i] > x[j] }
func (x byteSliceReverse) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

type rangeSlice []Range

var _ sort.Interface = (rangeSlice)(nil)

func (x rangeSlice) Len() int           { return len(x) }
func (x rangeSlice) Less(i, j int) bool { return x[i].Lo < x[j].Lo }
func (x rangeSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

func forEachByte(lo, hi byte, f func(b byte)) {
	for i := uint(lo); i <= uint(hi); i++ {
		f(byte(i))
	}
}

func forEachUnion(ms []Matcher, f func(b byte)) {
	if len(ms) == 0 {
		return
	}

	chans := make([]chan byte, len(ms))
	for i := range ms {
		ch := make(chan byte)
		m := ms[i]
		go func() {
			m.ForEach(func(b byte) { ch <- b })
			close(ch)
		}()
		chans[i] = ch
	}

	var data []byte
	seen := make(map[byte]struct{})
	for {
		for _, ch := range chans {
			for {
				b, ok := <-ch
				if !ok {
					break
				}
				_, found := seen[b]
				if !found {
					data = append(data, b)
					seen[b] = struct{}{}
					break
				}
			}
		}
		if len(data) == 0 {
			break
		}
		sort.Sort(byteSliceReverse(data))
		i := len(data) - 1
		f(data[i])
		data = data[:i]
	}
}

func forEachIntersection(ms []Matcher, f func(b byte)) {
	if len(ms) == 0 {
		forEachByte(0, 255, f)
		return
	}
	first := ms[0]
	rest := ms[1:]
	first.ForEach(func(b byte) {
		for _, sub := range rest {
			if !sub.Match(b) {
				return
			}
		}
		f(b)
	})
}

func genericForEach(m Matcher, f func(b byte)) {
	for i := uint(0); i < 256; i++ {
		if m.Match(byte(i)) {
			f(byte(i))
		}
	}
}

func genericString(m Matcher) string {
	var buf bytes.Buffer
	buf.WriteByte('[')
	m.ForEach(func(b byte) {
		fmt.Fprintf(&buf, "\\x%02x", b)
	})
	buf.WriteByte(']')
	return buf.String()
}
