package byteset

import (
	"regexp"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type matchRow struct {
	Input    byte
	Expected bool
}

func bytesAsRunes(in []byte) []rune {
	out := make([]rune, len(in))
	for i, b := range in {
		out[i] = rune(b)
	}
	return out
}

var allBytes []byte

func init() {
	allBytes = make([]byte, 256)
	for i := 0; i < 256; i++ {
		allBytes[i] = byte(i)
	}
}

func runByteMatchTests(t *testing.T, m Matcher, data []matchRow) {
	t.Helper()
	for i, row := range data {
		actual := m.Match(row.Input)
		if row.Expected != actual {
			t.Errorf("%s/%03d: %q: expected %v, got %v", t.Name(), i, row.Input, row.Expected, actual)
		}
	}
}

func runForEachTests(t *testing.T, m Matcher, expected []byte) {
	actual := make([]byte, 0, len(expected))
	m.ForEach(func(b byte) {
		actual = append(actual, b)
	})
	if string(actual) == string(expected) {
		return
	}
	actualRunes := bytesAsRunes(actual)
	expectedRunes := bytesAsRunes(expected)
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMainRunes(expectedRunes, actualRunes, false)
	pretty := dmp.DiffPrettyText(diffs)
	nl := regexp.MustCompile(`(?m)^`)
	pretty = nl.ReplaceAllLiteralString(pretty, "\t")
	t.Errorf("%s: wrong output:\n%s", t.Name(), pretty)
}

func TestAll_Match(t *testing.T) {
	m := All()
	runByteMatchTests(t, m, []matchRow{
		matchRow{'0', true},
		matchRow{'A', true},
		matchRow{'z', true},
		matchRow{' ', true},
		matchRow{0xff, true},
		matchRow{0x00, true},
		matchRow{0x99, true},
		matchRow{0xff, true},
	})
}

func TestAll_ForEach(t *testing.T) {
	m := All()
	runForEachTests(t, m, allBytes)
}

func TestAll_String(t *testing.T) {
	m := All()
	actual := m.String()
	expected := "."
	if expected != actual {
		t.Errorf("%s: expected %q, got %q", t.Name(), expected, actual)
	}
}

func TestNone_Match(t *testing.T) {
	m := None()
	runByteMatchTests(t, m, []matchRow{
		matchRow{'0', false},
		matchRow{'A', false},
		matchRow{'z', false},
		matchRow{' ', false},
		matchRow{0xff, false},
		matchRow{0x00, false},
		matchRow{0x99, false},
		matchRow{0xff, false},
	})
}

func TestNone_ForEach(t *testing.T) {
	m := None()
	runForEachTests(t, m, nil)
}

func TestNone_String(t *testing.T) {
	m := None()
	actual := m.String()
	expected := "!."
	if expected != actual {
		t.Errorf("%s: expected %q, got %q", t.Name(), expected, actual)
	}
}

func TestNegate_Match(t *testing.T) {
	m0 := Not(All())
	runByteMatchTests(t, m0, []matchRow{
		matchRow{'0', false},
		matchRow{'A', false},
		matchRow{'z', false},
		matchRow{' ', false},
		matchRow{0xff, false},
		matchRow{0x00, false},
		matchRow{0x99, false},
		matchRow{0xff, false},
	})

	m1 := Not(None())
	runByteMatchTests(t, m1, []matchRow{
		matchRow{'0', true},
		matchRow{'A', true},
		matchRow{'z', true},
		matchRow{' ', true},
		matchRow{0xff, true},
		matchRow{0x00, true},
		matchRow{0x99, true},
		matchRow{0xff, true},
	})
}

func TestNegate_ForEach(t *testing.T) {
	m0 := Not(All())
	runForEachTests(t, m0, nil)

	m1 := Not(None())
	runForEachTests(t, m1, allBytes)
}

func TestNegate_String(t *testing.T) {
	m := Not(All())
	actual := m.String()
	expected := "!."
	if expected != actual {
		t.Errorf("%s: expected %q, got %q", t.Name(), expected, actual)
	}
}

func TestIntersection_Match(t *testing.T) {
	m := And()
	runByteMatchTests(t, m, []matchRow{
		matchRow{0x00, true},
		matchRow{0x55, true},
		matchRow{0xff, true},
	})
	m = And(All())
	runByteMatchTests(t, m, []matchRow{
		matchRow{0x00, true},
		matchRow{0x55, true},
		matchRow{0xff, true},
	})
	m = And(All(), None())
	runByteMatchTests(t, m, []matchRow{
		matchRow{0x00, false},
		matchRow{0x55, false},
		matchRow{0xff, false},
	})
}

func TestUnion_Match(t *testing.T) {
	m := Or()
	runByteMatchTests(t, m, []matchRow{
		matchRow{0x00, false},
		matchRow{0x55, false},
		matchRow{0xff, false},
	})
	m = Or(None())
	runByteMatchTests(t, m, []matchRow{
		matchRow{0x00, false},
		matchRow{0x55, false},
		matchRow{0xff, false},
	})
	m = Or(None(), All())
	runByteMatchTests(t, m, []matchRow{
		matchRow{0x00, true},
		matchRow{0x55, true},
		matchRow{0xff, true},
	})
}

func makeSparseDemo() Matcher {
	return SparseSet('a', 'e', 'i', 'o', 'u')
}

func TestSparseSet_Match(t *testing.T) {
	m := makeSparseDemo()
	runByteMatchTests(t, m, []matchRow{
		matchRow{'a', true},
		matchRow{'e', true},
		matchRow{'i', true},
		matchRow{'o', true},
		matchRow{'u', true},
		matchRow{'9', false},
		matchRow{'b', false},
		matchRow{'f', false},
		matchRow{'z', false},
	})
}

func TestSparseSet_ForEach(t *testing.T) {
	m := makeSparseDemo()
	runForEachTests(t, m, []byte{'a', 'e', 'i', 'o', 'u'})
}

func makeDenseDemo() Matcher {
	return DenseSet('a', 'e', 'i', 'o', 'u')
}

func TestDenseSet_Match(t *testing.T) {
	m := makeDenseDemo()
	runByteMatchTests(t, m, []matchRow{
		matchRow{'a', true},
		matchRow{'e', true},
		matchRow{'i', true},
		matchRow{'o', true},
		matchRow{'u', true},
		matchRow{'9', false},
		matchRow{'b', false},
		matchRow{'f', false},
		matchRow{'z', false},
	})
}

func TestDenseSet_ForEach(t *testing.T) {
	m := makeDenseDemo()
	runForEachTests(t, m, []byte{'a', 'e', 'i', 'o', 'u'})
}

func makeRangeDemo() Matcher {
	return Ranges(
		Range{'0', '9'},
		Range{'A', 'Z'},
		Range{'a', 'z'})
}

func TestRange_Match(t *testing.T) {
	m := makeRangeDemo()
	runByteMatchTests(t, m, []matchRow{
		matchRow{'0', true},
		matchRow{'7', true},
		matchRow{'9', true},
		matchRow{'A', true},
		matchRow{'X', true},
		matchRow{'Z', true},
		matchRow{'a', true},
		matchRow{'x', true},
		matchRow{'z', true},
		matchRow{' ', false},
		matchRow{'@', false},
		matchRow{'`', false},
	})
}

func TestRange_ForEach(t *testing.T) {
	m := makeRangeDemo()
	runForEachTests(t, m, []byte{
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
		'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M',
		'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',
		'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm',
		'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
	})
}

func TestBytes(t *testing.T) {
	m0 := makeSparseDemo()
	actual := string(Bytes(m0, nil))
	expected := "aeiou"
	if actual != expected {
		t.Errorf("%s: expected %q, actual %q", t.Name(), expected, actual)
	}

	m1 := makeRangeDemo()
	actual = string(Bytes(m1, nil))
	expected = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	if actual != expected {
		t.Errorf("%s: expected %q, actual %q", t.Name(), expected, actual)
	}

	m2 := Or(m0, m1)
	actual = string(Bytes(m2, nil))
	expected = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	if actual != expected {
		t.Errorf("%s: expected %q, actual %q", t.Name(), expected, actual)
	}
}
