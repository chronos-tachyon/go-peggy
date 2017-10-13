package peggyvm

import (
	"bytes"
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
			dmp := diffmatchpatch.New()
			diffs := dmp.DiffMain(expected, actual, false)
			pretty := dmp.DiffPrettyText(diffs)
			lf := regexp.MustCompile(`(?m)^`)
			pretty = lf.ReplaceAllLiteralString(pretty, "\t")
			t.Errorf("%s/%03d: wrong output:\n%s", t.Name(), i, pretty)
			t.Log(expected)
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
