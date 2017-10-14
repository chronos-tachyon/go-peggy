package peggyvm

import (
	"bytes"
	"fmt"
	"regexp"
	"testing"

	"github.com/renstrom/dedent"
	"github.com/sergi/go-diff/diffmatchpatch"
)

var sampleProgram1 *Program
var sampleProgram2 *Program

func init() {
	// Regex:
	//
	//   `.*ana$`
	//
	// PEG:
	//
	//   main <- 'ana' !. / . main
	//
	// VM bytecode:
	//
	// 000 ac 40 00       BCAP 0
	// 003 14 07     .L0: CHOICE .L1 <.+7>
	// 005 64 00          LITB 0
	// 007 14 07          CHOICE .L2 <.+7>
	// 009 40             ANYB
	// 00a a6 00          FAIL2X
	// 00c 40        .L1: ANYB
	// 00d 90 40 f3       JMP .L0 <.-13>
	// 010 ae 40 00  .L2: ECAP 0
	// 013 fe 00          END
	//
	sampleProgram1 = &Program{
		Bytes: []byte{
			0xac, 0x40, 0x00,
			0x14, 0x07,
			0x64, 0x00,
			0x14, 0x07,
			0x40,
			0xa6, 0x00,
			0x40,
			0x90, 0x40, 0xf3,
			0xae, 0x40, 0x00,
			0xfe, 0x00,
		},
		Literals: [][]byte{
			[]byte{'a', 'n', 'a'},
		},
		Captures: []CaptureMeta{
			CaptureMeta{},
		},
		Labels: []*Label{
			&Label{0x03, false, ".L0"},
			&Label{0x0c, false, ".L1"},
			&Label{0x10, false, ".L2"},
		},
		LabelsByName: make(map[string]*Label),
	}
	for _, label := range sampleProgram1.Labels {
		sampleProgram1.LabelsByName[label.Name] = label
	}

	// Regex:
	//
	//   `^b(an)*a$`
	//
	// PEG:
	//
	//   main <- 'b' rest 'a' !.
	//   rest <- [ 'a' 'n' ] rest / ''
	//
	// VM bytecode:
	//
	// 000 ac 40 00          BCAP 0
	// 003 54 62             SAMEB 'b'
	// 005 14 0a        .L0: CHOICE .L1 <.+10>
	// 007 54 61             SAMEB 'a'
	// 009 54 6e             SAMEB 'n'
	// 00b aa 48 01 02       FCAP 1, 2
	// 00f 24 f4             COMMIT .L0 <.-12>
	// 011 54 61        .L1: SAMEB 'a'
	// 013 14 03             CHOICE .L2 <.+3>
	// 015 40                ANYB
	// 016 a6 00             FAIL2X
	// 018 ae 40 00     .L2: ECAP 0
	// 01b fe 00             END
	//
	sampleProgram2 = &Program{
		Bytes: []byte{
			0xac, 0x40, 0x00,
			0x54, 0x62,
			0x14, 0x0a,
			0x54, 0x61,
			0x54, 0x6e,
			0xaa, 0x48, 0x01, 0x02,
			0x24, 0xf4,
			0x54, 0x61,
			0x14, 0x03,
			0x40,
			0xa6, 0x00,
			0xae, 0x40, 0x00,
			0xfe, 0x00,
		},
		Captures: []CaptureMeta{
			CaptureMeta{},
			CaptureMeta{Repeat: true},
		},
		Labels: []*Label{
			&Label{0x05, false, ".L0"},
			&Label{0x11, false, ".L1"},
			&Label{0x18, false, ".L2"},
		},
		LabelsByName: make(map[string]*Label),
	}
	for _, label := range sampleProgram2.Labels {
		sampleProgram2.LabelsByName[label.Name] = label
	}
}

var reNL = regexp.MustCompile(`(?m)^`)

func diff(l, r string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(l, r, false)
	pretty := dmp.DiffPrettyText(diffs)
	return reNL.ReplaceAllLiteralString(pretty, "\t")
}

func TestProgram_Disassemble(t *testing.T) {
	type testrow struct {
		Program  *Program
		Expected string
	}

	data := []testrow{
		testrow{
			Program: sampleProgram1,
			Expected: `
			%literal "ana"
			%captures 1

				BCAP 0
			.L0:
				CHOICE .L1 <.+7>
				LITB 0
				CHOICE .L2 <.+7>
				ANYB
				FAIL2X
			.L1:
				ANYB
				JMP .L0 <.-13>
			.L2:
				ECAP 0
				END
			`,
		},
		testrow{
			Program: sampleProgram2,
			Expected: `
			%captures 2

				BCAP 0
				SAMEB 'b'
			.L0:
				CHOICE .L1 <.+10>
				SAMEB 'a'
				SAMEB 'n'
				FCAP 1, 2
				COMMIT .L0 <.-12>
			.L1:
				SAMEB 'a'
				CHOICE .L2 <.+3>
				ANYB
				FAIL2X
			.L2:
				ECAP 0
				END
			`,
		},
	}

	for i, row := range data {
		var buf bytes.Buffer
		_, err := row.Program.Disassemble(&buf)
		if err != nil {
			t.Errorf("%s/%03d: error: %v", t.Name(), i, err)
			continue
		}
		actual := buf.String()
		expected := dedent.Dedent(row.Expected)[1:]
		if actual != expected {
			t.Errorf("%s/%03d: wrong output:\n%s", t.Name(), i, diff(expected, actual))
		}
	}
}

func TestProgram_Match(t *testing.T) {
	type testrow struct {
		Program *Program
		Input   string
		Output  Result
	}

	data := []testrow{
		testrow{
			Program: sampleProgram1,
			Input:   "ana",
			Output: Result{
				Success: true,
				Captures: []Capture{
					Capture{
						Exists: true,
						Solo:   CapturePair{0, 3},
						Multi:  []CapturePair{CapturePair{0, 3}},
					},
				},
			},
		},
		testrow{
			Program: sampleProgram1,
			Input:   "anax",
			Output: Result{
				Success:  false,
				Captures: nil,
			},
		},
		testrow{
			Program: sampleProgram1,
			Input:   "banana",
			Output: Result{
				Success: true,
				Captures: []Capture{
					Capture{
						Exists: true,
						Solo:   CapturePair{0, 6},
						Multi:  []CapturePair{CapturePair{0, 6}},
					},
				},
			},
		},
		testrow{
			Program: sampleProgram1,
			Input:   "apple",
			Output: Result{
				Success:  false,
				Captures: nil,
			},
		},

		testrow{
			Program: sampleProgram2,
			Input:   "ba",
			Output: Result{
				Success: true,
				Captures: []Capture{
					Capture{
						Exists: true,
						Solo:   CapturePair{0, 2},
						Multi:  []CapturePair{CapturePair{0, 2}},
					},
					Capture{},
				},
			},
		},
		testrow{
			Program: sampleProgram2,
			Input:   "bana",
			Output: Result{
				Success: true,
				Captures: []Capture{
					Capture{
						Exists: true,
						Solo:   CapturePair{0, 4},
						Multi:  []CapturePair{CapturePair{0, 4}},
					},
					Capture{
						Exists: true,
						Solo:   CapturePair{1, 3},
						Multi:  []CapturePair{CapturePair{1, 3}},
					},
				},
			},
		},
		testrow{
			Program: sampleProgram2,
			Input:   "banana",
			Output: Result{
				Success: true,
				Captures: []Capture{
					Capture{
						Exists: true,
						Solo:   CapturePair{0, 6},
						Multi:  []CapturePair{CapturePair{0, 6}},
					},
					Capture{
						Exists: true,
						Solo:   CapturePair{3, 5},
						Multi:  []CapturePair{CapturePair{1, 3}, CapturePair{3, 5}},
					},
				},
			},
		},
		testrow{
			Program: sampleProgram2,
			Input:   "bx",
			Output: Result{
				Success:  false,
				Captures: nil,
			},
		},
		testrow{
			Program: sampleProgram2,
			Input:   "bax",
			Output: Result{
				Success:  false,
				Captures: nil,
			},
		},
		testrow{
			Program: sampleProgram2,
			Input:   "bananax",
			Output: Result{
				Success:  false,
				Captures: nil,
			},
		},
	}

	for i, row := range data {
		r := row.Program.Match([]byte(row.Input))
		actual := r.String()
		expected := row.Output.String()
		if actual != expected {
			t.Errorf("%s/%03d: wrong output:\n\texpected: %s\n\tactual: %s", t.Name(), i, expected, actual)
		}
	}
}

func TestImmMeta_Encode(t *testing.T) {
	m0 := ImmMeta{Type: ImmUint, Required: true}
	m1 := ImmMeta{Type: ImmUint, Required: false, PackedDefault: 0x01}
	m2 := ImmMeta{Type: ImmUint, Required: false, PackedDefault: 0xff}
	m3 := ImmMeta{Type: ImmSint, Required: true}
	m4 := ImmMeta{Type: ImmSint, Required: false, PackedDefault: 0x01}
	m5 := ImmMeta{Type: ImmSint, Required: false, PackedDefault: 0xff}

	type testrow struct {
		Meta     ImmMeta
		Value    uint64
		Expected []byte
	}

	data := []testrow{
		testrow{m0, 0x0000000000000000, []byte{0x00}},
		testrow{m0, 0x0000000000000001, []byte{0x01}},
		testrow{m0, 0x000000000000007f, []byte{0x7f}},
		testrow{m0, 0x0000000000000080, []byte{0x80}},
		testrow{m0, 0x00000000000000ff, []byte{0xff}},
		testrow{m0, 0x0000000000000100, []byte{0x00, 0x01}},
		testrow{m0, 0x0000000000007fff, []byte{0xff, 0x7f}},
		testrow{m0, 0x0000000000008000, []byte{0x00, 0x80}},
		testrow{m0, 0x000000000000ffff, []byte{0xff, 0xff}},
		testrow{m0, 0x0000000000010000, []byte{0x00, 0x00, 0x01, 0x00}},
		testrow{m0, 0x00000000ffffffff, []byte{0xff, 0xff, 0xff, 0xff}},
		testrow{m0, 0x0000000100000000, []byte{0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00}},
		testrow{m0, 0xffffffffffffffff, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},

		testrow{m1, 0x0000000000000000, []byte{0x00}},
		testrow{m1, 0x0000000000000001, nil},
		testrow{m1, 0x00000000000000fe, []byte{0xfe}},
		testrow{m1, 0x00000000000000ff, []byte{0xff}},

		testrow{m2, 0x0000000000000000, []byte{0x00}},
		testrow{m2, 0x0000000000000001, []byte{0x01}},
		testrow{m2, 0x00000000000000fe, []byte{0xfe}},
		testrow{m2, 0x00000000000000ff, nil},

		testrow{m3, 0x0000000000000000, []byte{0x00}},
		testrow{m3, 0x0000000000000001, []byte{0x01}},
		testrow{m3, 0x000000000000007f, []byte{0x7f}},
		testrow{m3, 0x0000000000000080, []byte{0x80, 0x00}},
		testrow{m3, 0x00000000000000ff, []byte{0xff, 0x00}},
		testrow{m3, 0xfffffffffffffffe, []byte{0xfe}},
		testrow{m3, 0xffffffffffffffff, []byte{0xff}},

		testrow{m4, 0x0000000000000000, []byte{0x00}},
		testrow{m4, 0x0000000000000001, nil},
		testrow{m4, 0x000000000000007f, []byte{0x7f}},
		testrow{m4, 0x0000000000000080, []byte{0x80, 0x00}},
		testrow{m4, 0x00000000000000ff, []byte{0xff, 0x00}},
		testrow{m4, 0xfffffffffffffffe, []byte{0xfe}},
		testrow{m4, 0xffffffffffffffff, []byte{0xff}},

		testrow{m5, 0x0000000000000000, []byte{0x00}},
		testrow{m5, 0x0000000000000001, []byte{0x01}},
		testrow{m5, 0x000000000000007f, []byte{0x7f}},
		testrow{m5, 0x0000000000000080, []byte{0x80, 0x00}},
		testrow{m5, 0x00000000000000ff, []byte{0xff, 0x00}},
		testrow{m5, 0xfffffffffffffffe, []byte{0xfe}},
		testrow{m5, 0xffffffffffffffff, nil},
	}

	for i, row := range data {
		expected := hexDump(row.Expected)
		actual := hexDump(row.Meta.Encode(row.Value))
		if expected != actual {
			t.Errorf("%s/%03d: wrong output:\n%s", t.Name(), i, diff(expected, actual))
		}
	}
}

func testAssemblerHelper(t *testing.T, a *Assembler, expected string) {
	t.Helper()

	p, err := a.Finish()
	if err != nil {
		t.Errorf("%s: error: %v", t.Name(), err)
		return
	}

	var buf bytes.Buffer
	buf.WriteString(hexDump(p.Bytes))
	for _, label := range p.Labels {
		fmt.Fprintf(&buf, "%q %t 0x%x\n", label.Name, label.Public, label.Offset)
	}

	actual := buf.String()
	expected = dedent.Dedent(expected)[1:]
	if expected != actual {
		t.Errorf("%s: wrong output:\n%s", t.Name(), diff(expected, actual))
	}
}

func TestAssembler_one(t *testing.T) {
	a := NewAssembler()
	a.DeclareNumCaptures(1)
	a.EmitOp(OpBCAP.Meta(), 0, nil, nil)
	a.EmitOp(OpECAP.Meta(), 0, nil, nil)
	a.EmitOp(OpEND.Meta(), nil, nil, nil)

	testAssemblerHelper(t, a, `
	00000  ac 40 00 ae 40 00 fe 00
	00008
	`)
}

func TestAssembler_two(t *testing.T) {
	a := NewAssembler()
	a.DeclareNumCaptures(1)
	a.EmitOp(OpBCAP.Meta(), 0, nil, nil)
	a.EmitLabel(".L0")
	a.EmitOp(OpCHOICE.Meta(), a.GrabLabel(".L1"), nil, nil)
	a.EmitOp(OpSAMEB.Meta(), 'x', nil, nil)
	a.EmitOp(OpCHOICE.Meta(), a.GrabLabel(".L2"), nil, nil)
	a.EmitOp(OpANYB.Meta(), nil, nil, nil)
	a.EmitOp(OpFAIL2X.Meta(), nil, nil, nil)
	a.EmitLabel(".L1")
	a.EmitOp(OpANYB.Meta(), nil, nil, nil)
	a.EmitOp(OpJMP.Meta(), a.GrabLabel(".L0"), nil, nil)
	a.EmitLabel(".L2")
	a.EmitOp(OpECAP.Meta(), 0, nil, nil)
	a.EmitOp(OpEND.Meta(), nil, nil, nil)

	testAssemblerHelper(t, a, `
	00000  ac 40 00 14 07 54 78 14  07 40 a6 00 40 90 40 f3
	00010  ae 40 00 fe 00
	00015
	".L0" false 0x3
	".L1" false 0xc
	".L2" false 0x10
	`)
}

func TestAssembler_three(t *testing.T) {
	a := NewAssembler()
	a.DeclareNumCaptures(2)
	a.EmitOp(OpBCAP.Meta(), 0, nil, nil)
	a.EmitOp(OpSAMEB.Meta(), 'b', nil, nil)
	a.EmitLabel(".L0")
	a.EmitOp(OpCHOICE.Meta(), a.GrabLabel(".L1"), nil, nil)
	a.EmitOp(OpSAMEB.Meta(), 'a', nil, nil)
	a.EmitOp(OpSAMEB.Meta(), 'n', nil, nil)
	a.EmitOp(OpFCAP.Meta(), 1, 2, nil)
	a.EmitOp(OpCOMMIT.Meta(), a.GrabLabel(".L0"), nil, nil)
	a.EmitLabel(".L1")
	a.EmitOp(OpSAMEB.Meta(), 'a', nil, nil)
	a.EmitOp(OpCHOICE.Meta(), a.GrabLabel(".L2"), nil, nil)
	a.EmitOp(OpANYB.Meta(), nil, nil, nil)
	a.EmitOp(OpFAIL2X.Meta(), nil, nil, nil)
	a.EmitLabel(".L2")
	a.EmitOp(OpECAP.Meta(), 0, nil, nil)
	a.EmitOp(OpEND.Meta(), nil, nil, nil)

	testAssemblerHelper(t, a, `
	00000  ac 40 00 54 62 14 0a 54  61 54 6e aa 48 01 02 24
	00010  f4 54 61 14 03 40 a6 00  ae 40 00 fe 00
	0001d
	".L0" false 0x5
	".L1" false 0x11
	".L2" false 0x18
	`)
}

func TestAssembler_four(t *testing.T) {
	a := NewAssembler()
	a.DeclareNumCaptures(0)
	a.EmitLabel(".L0")
	a.EmitOp(OpJMP.Meta(), a.GrabLabel(".L0"), nil, nil)

	testAssemblerHelper(t, a, `
	00000  90 40 fd
	00003
	".L0" false 0x0
	`)
}

func TestAssembler_five(t *testing.T) {
	a := NewAssembler()
	a.DeclareNumCaptures(0)
	a.EmitLabel(".L0")
	a.EmitOp(OpNOP.Meta(), nil, nil, nil)
	a.EmitOp(OpNOP.Meta(), nil, nil, nil)
	a.EmitOp(OpNOP.Meta(), nil, nil, nil)
	a.EmitOp(OpJMP.Meta(), a.GrabLabel(".L0"), nil, nil)

	testAssemblerHelper(t, a, `
	00000  00 00 00 90 40 fa
	00006
	".L0" false 0x0
	`)
}

func TestAssembler_six(t *testing.T) {
	a := NewAssembler()
	a.DeclareNumCaptures(0)
	a.EmitLabel(".L0")
	for i := 0; i < 0x7d; i++ {
		a.EmitOp(OpNOP.Meta(), nil, nil, nil)
	}
	a.EmitOp(OpJMP.Meta(), a.GrabLabel(".L0"), nil, nil)

	testAssemblerHelper(t, a, `
	00000  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00010  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00020  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00030  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00040  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00050  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00060  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00070  00 00 00 00 00 00 00 00  00 00 00 00 00 90 40 80
	00080
	".L0" false 0x0
	`)
}

func TestAssembler_seven(t *testing.T) {
	a := NewAssembler()
	a.DeclareNumCaptures(0)
	a.EmitLabel(".L0")
	for i := 0; i < 0x7e; i++ {
		a.EmitOp(OpNOP.Meta(), nil, nil, nil)
	}
	a.EmitOp(OpJMP.Meta(), a.GrabLabel(".L0"), nil, nil)

	testAssemblerHelper(t, a, `
	00000  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00010  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00020  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00030  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00040  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00050  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00060  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00070  00 00 00 00 00 00 00 00  00 00 00 00 00 00 90 80
	00080  7e ff
	00082
	".L0" false 0x0
	`)
}

func TestAssembler_eight(t *testing.T) {
	a := NewAssembler()
	a.DeclareNumCaptures(0)
	a.EmitOp(OpJMP.Meta(), a.GrabLabel(".L0"), nil, nil)
	for i := 0; i < 0x7f; i++ {
		a.EmitOp(OpNOP.Meta(), nil, nil, nil)
	}
	a.EmitLabel(".L0")

	testAssemblerHelper(t, a, `
	00000  90 40 7f 00 00 00 00 00  00 00 00 00 00 00 00 00
	00010  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00020  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00030  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00040  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00050  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00060  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00070  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00080  00 00
	00082
	".L0" false 0x82
	`)
}

func TestAssembler_nine(t *testing.T) {
	a := NewAssembler()
	a.DeclareNumCaptures(0)
	a.EmitOp(OpJMP.Meta(), a.GrabLabel(".L0"), nil, nil)
	for i := 0; i < 0x80; i++ {
		a.EmitOp(OpNOP.Meta(), nil, nil, nil)
	}
	a.EmitLabel(".L0")

	testAssemblerHelper(t, a, `
	00000  90 80 80 00 00 00 00 00  00 00 00 00 00 00 00 00
	00010  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00020  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00030  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00040  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00050  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00060  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00070  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
	00080  00 00 00 00
	00084
	".L0" false 0x84
	`)
}
