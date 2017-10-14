package peggyvm

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"unicode/utf8"

	"github.com/chronos-tachyon/go-peggy/byteset"
)

// Program is a PEG pattern that has been compiled to bytecode.
type Program struct {
	// Bytes is the bytecode to execute.
	Bytes []byte

	// Literals is a list of byte literals, referenced by the LITB / TLITB
	// family of instructions.
	Literals [][]byte

	// ByteSets is a list of matchers for byte sets, referenced by the
	// MATCHB / TMATCHB / SPANB family of instructions.
	ByteSets []byteset.Matcher

	// Captures is the list of all captures.
	//
	// - The whole match is always capture index 0.
	//
	// - The N user-specified captures are capture indices 1 .. N.
	//
	Captures []CaptureMeta

	// NamedCaptures is a map from capture names to capture indices.
	NamedCaptures map[string]uint64

	// Labels is an auxiliary list of program labels.
	Labels []*Label

	// LabelsByName is an index from Label.Name to Label.
	LabelsByName map[string]*Label
}

// FindLabel returns the best available label for the given code address. If no
// labels are defined for that code address, then a synthetic local label is
// returned.
func (p *Program) FindLabel(xp uint64) *Label {
	i := sort.Search(len(p.Labels), func(i int) bool {
		return p.Labels[i].Offset >= xp
	})
	if i < len(p.Labels) && p.Labels[i].Offset == xp {
		return p.Labels[i]
	}
	return &Label{
		Offset: xp,
		Public: false,
		Name:   fmt.Sprintf(".ANON@%x", xp),
	}
}

// Disassemble converts the program's bytecode into assembly instructions,
// writing the result to the provided buffer.
//
func (p *Program) Disassemble(w io.Writer) (int, error) {
	var buf bytes.Buffer
	var total int

	flush := func() error {
		n, err := w.Write(buf.Bytes())
		total += n
		buf.Reset()
		return err
	}

	for _, literal := range p.Literals {
		buf.WriteString("%literal ")
		if utf8.Valid(literal) {
			fmt.Fprintf(&buf, "%q", literal)
		} else {
			first := true
			for _, b := range literal {
				if !first {
					buf.WriteByte(',')
					buf.WriteByte(' ')
				}
				fmt.Fprintf(&buf, "0x%02x", b)
				first = false
			}
		}
		buf.WriteByte('\n')
		if err := flush(); err != nil {
			return total, err
		}
	}

	for _, matcher := range p.ByteSets {
		buf.WriteString("%matcher ")
		buf.WriteString(matcher.String())
		buf.WriteByte('\n')
		if err := flush(); err != nil {
			return total, err
		}
	}

	fmt.Fprintf(&buf, "%%captures %d\n", len(p.Captures))
	if err := flush(); err != nil {
		return total, err
	}
	for i, capture := range p.Captures {
		if capture.Name != "" {
			fmt.Fprintf(&buf, "%%namedcapture %d %q\n", i, capture.Name)
			if err := flush(); err != nil {
				return total, err
			}
		}
	}

	buf.WriteByte('\n')
	if err := flush(); err != nil {
		return total, err
	}

	var op Op
	var xp uint64

	// First pass: identify code offsets that need labels
	var labelNeeded = make(map[uint64]struct{})
	for {
		err := op.Decode(p.Bytes, xp)
		if err == io.EOF {
			break
		}
		if err != nil {
			return total, err
		}

		meta := op.Meta
		if meta == nil {
			meta = op.Code.Meta()
		}

		xp += uint64(op.Len)
		if meta.Imm0.Type == ImmCodeOffset {
			target := addOffset(xp, u2s(op.Imm0))
			labelNeeded[target] = struct{}{}
		}
		if meta.Imm1.Type == ImmCodeOffset {
			target := addOffset(xp, u2s(op.Imm1))
			labelNeeded[target] = struct{}{}
		}
		if meta.Imm2.Type == ImmCodeOffset {
			target := addOffset(xp, u2s(op.Imm2))
			labelNeeded[target] = struct{}{}
		}
	}

	// Second pass: generate actual disassembly listing
	xp = 0
	for {
		err := op.Decode(p.Bytes, xp)
		if err == io.EOF {
			break
		}
		if err != nil {
			return total, err
		}

		if _, yes := labelNeeded[xp]; yes {
			label := p.FindLabel(xp)
			if label != nil {
				buf.WriteString(label.Name)
				buf.WriteByte(':')
				buf.WriteByte('\n')
				if err := flush(); err != nil {
					return total, err
				}
			}
		}

		xp += uint64(op.Len)
		buf.WriteByte('\t')
		p.writeOp(&buf, &op, xp)
		buf.WriteByte('\n')
		if err := flush(); err != nil {
			return total, err
		}
	}
	return total, nil
}

func (p *Program) writeOp(buf *bytes.Buffer, op *Op, xp uint64) {
	meta := op.Meta
	if meta == nil {
		meta = op.Code.Meta()
	}

	first := true
	f := func(m ImmMeta, v uint64) {
		if !m.IsPresent(v) {
			return
		}
		if !first {
			buf.WriteByte(',')
		}
		buf.WriteByte(' ')
		first = false
		switch m.Type {
		case ImmUint, ImmCount:
			fmt.Fprintf(buf, "%d", v)

		case ImmSint:
			fmt.Fprintf(buf, "%d", u2s(v))

		case ImmByte:
			writeByteLiteral(buf, byte(v))

		case ImmRune:
			writeRuneLiteral(buf, rune(v))

		case ImmCodeOffset:
			s := u2s(v)
			label := p.FindLabel(addOffset(xp, s))
			fmt.Fprintf(buf, "%s <.%+d>", label.Name, s)

		case ImmLiteralIdx:
			fmt.Fprintf(buf, "%d", v)
			if v >= uint64(len(p.Literals)) {
				buf.WriteString(" <bad-literal>")
			}

		case ImmMatcherIdx:
			fmt.Fprintf(buf, "%d", v)
			if v >= uint64(len(p.ByteSets)) {
				buf.WriteString(" <bad-matcher>")
			}

		case ImmCaptureIdx:
			fmt.Fprintf(buf, "%d", v)
			if v >= uint64(len(p.Captures)) {
				buf.WriteString(" <bad-capture>")
			}

		default:
			fmt.Fprintf(buf, "%d", v)
		}
	}

	buf.WriteString(meta.Name)
	f(meta.Imm0, op.Imm0)
	f(meta.Imm1, op.Imm1)
	f(meta.Imm2, op.Imm2)
}

func (p *Program) String() string {
	var buf bytes.Buffer
	buf.WriteString("Program{")
	// FIXME
	buf.WriteString("}")
	return buf.String()
}

func (p *Program) Exec(input []byte) *Execution {
	ks := make([]Assignment, 0, 2*len(p.Captures))
	cs := make([]Frame, 0, 16)
	return &Execution{
		P:  p,
		I:  input,
		DP: 0,
		XP: 0,
		KS: ks,
		CS: cs,
	}
}

func (p *Program) Match(input []byte) Result {
	var r Result
	x := p.Exec(input)
	if err := x.Run(); err != nil {
		panic(err)
	}
	r.Success = (x.R == SuccessState)
	r.Captures = make([]Capture, len(p.Captures))
	pending := make([]uint64, len(p.Captures))
	for _, a := range x.KS {
		if a.Index >= uint64(len(r.Captures)) {
			panic("capture out of range")
		}
		if a.IsEnd {
			var pair CapturePair
			pair.S = pending[a.Index]
			pair.E = a.DP
			ptr := &r.Captures[a.Index]
			ptr.Exists = true
			ptr.Solo = pair
			ptr.Multi = append(ptr.Multi, pair)
			pending[a.Index] = 0
		} else {
			pending[a.Index] = a.DP
		}
	}
	return r
}
