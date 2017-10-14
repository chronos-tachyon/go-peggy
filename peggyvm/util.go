package peggyvm

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"unicode"
)

type byCode []OpMeta

var _ sort.Interface = (byCode)(nil)

func (x byCode) Len() int           { return len(x) }
func (x byCode) Less(i, j int) bool { return x[i].Code < x[j].Code }
func (x byCode) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

// ImmLengthDecode returns the decoded length of an immediate. The argument is
// a 3-bit value, aligned to the LSB of a byte.
func ImmLengthDecode(b byte) (length uint, valid bool) {
	switch b {
	case 0:
		return 0, true

	case 1:
		return 1, true

	case 2:
		return 2, true

	case 3:
		return 4, true

	case 4:
		return 8, true
	}
	return 0, false
}

// ImmLengthEncode returns the encoded length of an immediate. The result is a
// 3-bit value, aligned to the LSB of a byte.
//
// This function will panic if n ∉ {0, 1, 2, 4, 8}.
//
func ImmLengthEncode(n int) byte {
	switch n {
	case 0:
		return 0
	case 1:
		return 1
	case 2:
		return 2
	case 4:
		return 3
	case 8:
		return 4
	}
	panic("invalid immediate length")
}

// assert panics if cond is false.
func assert(cond bool, format string, args ...interface{}) {
	if !cond {
		var buf bytes.Buffer
		buf.WriteString("assertion failed: ")
		fmt.Fprintf(&buf, format, args...)
		panic(errors.New(buf.String()))
	}
}

// s2u converts an int64 to a 2's complement uint64.
func s2u(v int64) uint64 {
	if v < 0 {
		return ^(uint64(-v) - 1)
	}
	return uint64(v)
}

// u2s converts a 2's complement uint64 to an int64.
func u2s(v uint64) int64 {
	if (v & highbit) == highbit {
		return -int64(^v + 1)
	}
	return int64(v)
}

// addOffset calculates `xp + s` with overflow checking.
//
// This function will panic if overflow is detected.
//
func addOffset(xp uint64, s int64) uint64 {
	if s < 0 {
		if uint64(-s) > xp {
			panic("code offset out of range")
		}
		xp -= uint64(-s)
	} else {
		if uint64(s) > allbits-xp {
			panic("code offset out of range")
		}
		xp += uint64(s)
	}
	return xp
}

func writeByteLiteral(buf *bytes.Buffer, b byte) {
	if ctrl, found := wellKnownControls[rune(b)]; found {
		buf.WriteByte('\'')
		buf.WriteByte('\\')
		buf.WriteByte(ctrl)
		buf.WriteByte('\'')
	} else if b == '\\' || b == '\'' {
		buf.WriteByte('\'')
		buf.WriteByte('\\')
		buf.WriteByte(b)
		buf.WriteByte('\'')
	} else if b >= 0x20 && b < 0x7f {
		buf.WriteByte('\'')
		buf.WriteByte(b)
		buf.WriteByte('\'')
	} else {
		fmt.Fprintf(buf, "$%02x", b)
	}
}

func writeRuneLiteral(buf *bytes.Buffer, r rune) {
	if ctrl, found := wellKnownControls[r]; found {
		buf.WriteByte('\'')
		buf.WriteByte('\\')
		buf.WriteByte(ctrl)
		buf.WriteByte('\'')
	} else if r == '\\' || r == '\'' {
		buf.WriteByte('\'')
		buf.WriteByte('\\')
		buf.WriteRune(r)
		buf.WriteByte('\'')
	} else if unicode.IsPrint(r) {
		buf.WriteByte('\'')
		buf.WriteRune(r)
		buf.WriteByte('\'')
	} else {
		fmt.Fprintf(buf, "$%04x", r)
	}
}

func hexDump(in []byte) string {
	var buf bytes.Buffer
	buf.WriteString("00000")
	dirty := false
	i := uint(0)
	for i < uint(len(in)) {
		b := in[i]
		mod16 := i & 0xf
		if (mod16 == 0x0 || mod16 == 0x8) {
			buf.WriteByte(' ')
			buf.WriteByte(' ')
		} else {
			buf.WriteByte(' ')
		}
		fmt.Fprintf(&buf, "%02x", b)
		dirty = true
		i += 1
		if mod16 == 0xf {
			fmt.Fprintf(&buf, "\n%05x", i)
			dirty = false
		}
	}
	if dirty {
		fmt.Fprintf(&buf, "\n%05x", i)
	}
	buf.WriteByte('\n')
	return buf.String()
}
