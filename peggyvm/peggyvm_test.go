package peggyvm

import (
	"bytes"
	"testing"
	"regexp"

	"github.com/renstrom/dedent"
	"github.com/sergi/go-diff/diffmatchpatch"
)

func makeSampleProgram1() *Program {
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

	p := &Program{
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
	for _, label := range p.Labels {
		p.LabelsByName[label.Name] = label
	}
	return p
}

func TestDisassemble(t *testing.T) {
	p := makeSampleProgram1()

	var buf bytes.Buffer
	err := p.Disassemble(&buf)
	if err != nil {
		t.Errorf("%s: error: %v", t.Name(), err)
		return
	}

	actual := buf.String()
	expected := dedent.Dedent(`
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
	`)[1:]
	if actual != expected {
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(expected, actual, false)
		pretty := dmp.DiffPrettyText(diffs)
		lf := regexp.MustCompile(`(?m)^`)
		pretty = lf.ReplaceAllLiteralString(pretty, "\t")
		t.Errorf("%s: wrong output:\n%s", t.Name(), pretty)
		t.Log(expected)
	}
}

func TestProgram_Match(t *testing.T) {
	p := makeSampleProgram1()

	type testrow struct {
		Input  string
		Output Result
	}

	data := []testrow{
		testrow{
			Input: "ana",
			Output: Result{
				Success: true,
				Captures: []Capture{
					Capture{
						Solo: CapturePair{0, 3},
						Multi: []CapturePair{CapturePair{0, 3}},
					},
				},
			},
		},
		testrow{
			Input: "anax",
			Output: Result{
				Success: false,
				Captures: nil,
			},
		},
		testrow{
			Input: "banana",
			Output: Result{
				Success: true,
				Captures: []Capture{
					Capture{
						Solo: CapturePair{0, 6},
						Multi: []CapturePair{CapturePair{0, 6}},
					},
				},
			},
		},
		testrow{
			Input: "apple",
			Output: Result{
				Success: false,
				Captures: nil,
			},
		},
	}

	for i, row := range data {
		r := p.Match([]byte(row.Input))
		actual := r.String()
		expected := row.Output.String()
		if actual != expected {
			t.Errorf("%s/%03d: wrong output:\n\texpected: %s\n\tactual: %s", t.Name(), i, expected, actual)
		}
	}
}
