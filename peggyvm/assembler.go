package peggyvm

import (
	"bytes"
	"fmt"

	"github.com/chronos-tachyon/go-peggy/byteset"
)

// Assembler turns sequences of instructions into Program objects.
type Assembler struct {
	// List is the list of instructions and labels being assembled.
	List         []*AsmItem
	LabelsByName map[string]*AsmItem

	// Literals holds the future Program.Literals list.
	Literals [][]byte

	// ByteSets holds the future Program.ByteSets list.
	ByteSets []byteset.Matcher

	// Captures holds the future Program.Captures list.
	Captures      []CaptureMeta
	NamedCaptures map[string]uint64

	Queue []*AsmItem
}

type AsmItem struct {
	// Index is the index of this item within Assembler.List.
	Index uint

	// IsOp is true for instructions, false for labels.
	IsOp bool

	// XP is the absolute code address of (a) this label, or (b) the start
	// of this instruction. Only available if all prior instructions and
	// labels have been fixed.
	KnownXP bool
	XP      uint64

	// Name and Public contain information about the label.
	Name   string
	Public bool
	Seen   bool

	// Meta, Imm0, Imm1, and Imm2 contain information about the op.
	Meta *OpMeta
	Imm0 uint64
	Imm1 uint64
	Imm2 uint64

	// Fixed is true iff this label/op has been fixed.
	Fixed bool

	// Bytes holds the actual bytes iff this op has been fixed, nil otherwise.
	Bytes []byte

	// MaxLength hold this op's max encoded length.
	MaxLength uint

	// Fixup points to one of this op's Imm[012] slots, indicating which
	// one should be modified when fixing this op.
	Fixup *uint64

	Blocking     []*AsmItem
	FixBlockedBy *AsmItem
}

func NewAssembler() *Assembler {
	return &Assembler{
		LabelsByName:  make(map[string]*AsmItem),
		NamedCaptures: make(map[string]uint64),
	}
}

func (a *Assembler) DeclareLiteral(lit []byte) {
	a.Literals = append(a.Literals, lit)
}

func (a *Assembler) DeclareByteSet(set byteset.Matcher) {
	a.ByteSets = append(a.ByteSets, set)
}

func (a *Assembler) DeclareNumCaptures(n uint64) {
	a.Captures = make([]CaptureMeta, n)
}

func (a *Assembler) DeclareNamedCapture(idx uint64, name string) {
	assert(idx < uint64(len(a.Captures)), "capture index out of range")
	a.NamedCaptures[name] = idx
}

func (a *Assembler) GrabLabel(name string) *AsmItem {
	item := a.LabelsByName[name]
	if item != nil {
		return item
	}
	assert(len(name) != 0, "empty label name")
	public := true
	if name[0] == '.' {
		public = false
	}
	item = &AsmItem{
		Index:  ^uint(0),
		IsOp:   false,
		Name:   name,
		Public: public,
		Fixed:  true,
	}
	a.LabelsByName[name] = item
	return item
}

func (a *Assembler) EmitLabel(name string) {
	item := a.GrabLabel(name)
	item.Seen = true
	a.link(item)
}

func (a *Assembler) EmitOp(meta *OpMeta, imm0, imm1, imm2 interface{}) {
	item := &AsmItem{
		Index:     ^uint(0),
		IsOp:      true,
		Meta:      meta,
		Name:      meta.Name,
		MaxLength: 26,
	}

	type tuple struct {
		Meta  *ImmMeta
		Value interface{}
		Ptr   *uint64
	}

	tuples := []tuple{
		tuple{&meta.Imm0, imm0, &item.Imm0},
		tuple{&meta.Imm1, imm1, &item.Imm1},
		tuple{&meta.Imm2, imm2, &item.Imm2},
	}

	variableLen := false
	for _, row := range tuples {
		t := row.Meta.Type
		switch x := row.Value.(type) {
		case nil:
			assert(t == ImmNone || !row.Meta.Required, "nil for required immediate")
			*row.Ptr = row.Meta.Default()

		case uint:
			assert(!t.Signed(), "%T for signed immediate", x)
			*row.Ptr = uint64(x)

		case uint8:
			assert(!t.Signed(), "%T for signed immediate", x)
			*row.Ptr = uint64(x)

		case uint16:
			assert(!t.Signed(), "%T for signed immediate", x)
			*row.Ptr = uint64(x)

		case uint32:
			assert(!t.Signed(), "%T for signed immediate", x)
			*row.Ptr = uint64(x)

		case uint64:
			assert(!t.Signed(), "%T for signed immediate", x)
			*row.Ptr = x

		case int:
			if t.Signed() {
				*row.Ptr = s2u(int64(x))
			} else {
				assert(x >= 0, "negative value for unsigned immediate")
				*row.Ptr = uint64(x)
			}

		case int8:
			assert(t.Signed(), "%T for unsigned immediate", x)
			*row.Ptr = s2u(int64(x))

		case int16:
			assert(t.Signed(), "%T for unsigned immediate", x)
			*row.Ptr = s2u(int64(x))

		case int32:
			// Special handling for rune
			if t.Signed() {
				*row.Ptr = s2u(int64(x))
			} else {
				assert(x >= 0, "negative value for unsigned immediate")
				*row.Ptr = uint64(x)
			}

		case int64:
			assert(t.Signed(), "%T for unsigned immediate", x)
			*row.Ptr = s2u(x)

		case *AsmItem:
			assert(t == ImmCodeOffset, "not a code offset")
			assert(!x.IsOp, "not a label")
			assert(item.Fixup == nil, "multiple fixups for one op")
			variableLen = true
			item.Fixup = row.Ptr
			item.FixBlockedBy = x

		default:
			panic(fmt.Errorf("illegal type %T", x))
		}
	}

	a.link(item)

	if !variableLen {
		item.generate()
		return
	}

	label := item.FixBlockedBy
	label.Blocking = append(label.Blocking, item)
	*item.Fixup = ^highbit
	raw := meta.Encode(item.Imm0, item.Imm1, item.Imm2)
	item.MaxLength = uint(len(raw))
}

func (a *Assembler) Finish() (*Program, error) {
	a.Fix()

	var endxp uint64
	if len(a.List) != 0 {
		last := a.List[len(a.List)-1]
		endxp = last.XP + uint64(len(last.Bytes))
	}

	p := &Program{
		Bytes:         make([]byte, 0, endxp),
		Literals:      a.Literals,
		ByteSets:      a.ByteSets,
		Captures:      a.Captures,
		NamedCaptures: a.NamedCaptures,
		LabelsByName:  make(map[string]*Label),
	}

	for _, item := range a.List {
		if item.IsOp {
			p.Bytes = append(p.Bytes, item.Bytes...)
		} else {
			label := &Label{
				Name:   item.Name,
				Public: item.Public,
				Offset: item.XP,
			}
			p.Labels = append(p.Labels, label)
			p.LabelsByName[label.Name] = label
		}
	}

	return p, nil
}

func (a *Assembler) Fix() {
	a.Queue = make([]*AsmItem, 0, len(a.List))

	// First, try logically reasoning out all the lengths and positions.
	for {
		a.Queue = append(a.Queue, a.List...)
		progress := a.process()
		if !progress {
			break
		}
	}

	// Last resort: start jiggling the cables until it works.
	for _, item := range a.List {
		if item.Fixed {
			continue
		}

		n, _ := a.distance(item, item.FixBlockedBy)
		item.applyFixup(n)

		// Special consideration: negative offsets are affected by the
		// encoded length of the instruction itself. This produces edge
		// cases that are tricky to resolve optimally.
		if item.Index > item.FixBlockedBy.Index {
			first := item.Meta.Encode(item.Imm0, item.Imm1, item.Imm2)
			item.applyFixup(n + 1)
			second := item.Meta.Encode(item.Imm0, item.Imm1, item.Imm2)
			if len(second) == len(first) {
				item.applyFixup(n)
			}
		}

		item.generate()
	}

	// Now that all lengths are determined, calculate positions.
	for {
		a.Queue = append(a.Queue, a.List...)
		progress := a.process()
		if !progress {
			break
		}
	}

	for _, item := range a.List {
		assert(item.KnownXP && item.Fixed, "I done goofed: [%s]", item)
	}
}

func (a *Assembler) String() string {
	var buf bytes.Buffer
	for _, item := range a.List {
		buf.WriteString(item.String())
		buf.WriteByte('\n')
	}
	return buf.String()
}

func (item *AsmItem) String() string {
	var buf bytes.Buffer
	if item.KnownXP {
		fmt.Fprintf(&buf, "%05x ", item.XP)
	} else {
		buf.WriteString("    - ")
	}
	fmt.Fprintf(&buf, "#%02d ", item.Index)
	if item.Fixed {
		fmt.Fprintf(&buf, "%02x    ", len(item.Bytes))
	} else {
		fmt.Fprintf(&buf, "??/%02x ", item.MaxLength)
	}
	buf.WriteString(item.Name)
	if item.FixBlockedBy != nil {
		buf.WriteByte(' ')
		buf.WriteString(item.FixBlockedBy.Name)
	}
	return buf.String()
}

func (a *Assembler) link(item *AsmItem) {
	assert(item.Index == ^uint(0), "item used twice")
	item.Index = uint(len(a.List))
	a.List = append(a.List, item)
}

func (item *AsmItem) applyFixup(s int64) {
	assert(item.IsOp, "must be an op")
	assert(!item.Fixed, "must be waiting on a fix")
	assert(item.FixBlockedBy != nil, "FixBlockedBy is nil")
	*item.Fixup = s2u(s)
}

func (item *AsmItem) generate() {
	item.Bytes = item.Meta.Encode(item.Imm0, item.Imm1, item.Imm2)
	item.MaxLength = uint(len(item.Bytes))
	item.Fixed = true
	item.Fixup = nil
	item.FixBlockedBy = nil
}

func (a *Assembler) trySetXP(item *AsmItem) bool {
	if item.KnownXP {
		return false
	}

	if item.Index == 0 {
		item.XP = 0
		item.KnownXP = true
		return true
	}

	prev := a.List[item.Index-1]
	if prev.KnownXP && prev.Fixed {
		item.XP = prev.XP + uint64(len(prev.Bytes))
		item.KnownXP = true
		return true
	} else {
		prev.Blocking = append(prev.Blocking, item)
	}
	return false
}

func (a *Assembler) tryFix(item *AsmItem) bool {
	if item.Fixed {
		return false
	}

	label := item.FixBlockedBy
	if !label.Seen {
		return false
	}

	n, exact := a.distance(item, label)
	item.applyFixup(n)
	if exact {
		item.generate()
		return true
	}

	raw := item.Meta.Encode(item.Imm0, item.Imm1, item.Imm2)
	ml := uint(len(raw))
	if ml < item.MaxLength {
		item.MaxLength = ml
		return true
	}
	assert(ml == item.MaxLength, "max length of %s grew", item)
	return false
}

func (a *Assembler) processItem(item *AsmItem) bool {
	var prog0, prog1 bool
	if !item.KnownXP || !item.Fixed {
		prog0 = a.trySetXP(item)
		prog1 = a.tryFix(item)
	}
	if len(item.Blocking) != 0 {
		list := item.Blocking
		item.Blocking = nil
		a.Queue = append(a.Queue, list...)
	}
	return prog0 || prog1
}

func (a *Assembler) process() bool {
	progress := false
	for len(a.Queue) != 0 {
		item := a.Queue[0]
		a.Queue[0] = nil
		a.Queue = a.Queue[1:]
		if a.processItem(item) {
			progress = true
		}
	}
	return progress
}

// distance measures the distance between the *end* of p and the *start* of q.
func (a *Assembler) distance(p, q *AsmItem) (int64, bool) {
	i := p.Index + 1
	j := q.Index
	if i >= uint(len(a.List)) {
		a.List = append(a.List, &AsmItem{
			Index:  i,
			IsOp:   false,
			Name:   ".$bogus$",
			Public: false,
			Fixed:  true,
		})
		defer func() {
			a.List = a.List[:len(a.List)-1]
		}()
	}

	total := uint64(0)
	exact := true

	f := func() {
		item := a.List[i]
		if item.Fixed {
			total += uint64(len(item.Bytes))
		} else {
			total += uint64(item.MaxLength)
			exact = false
		}
	}

	var n int64
	if i > j {
		for i != j {
			i -= 1
			f()
		}
		n = -int64(total)
	} else {
		for i != j {
			f()
			i += 1
		}
		n = int64(total)
	}
	return n, exact
}
