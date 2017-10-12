package peggyvm

import (
	"sort"
)

const (
	allbits = ^uint64(0)
	highbit = uint64(1) << 63
)

var wellKnownControls = map[rune]byte{
	0x07: 'a',
	0x08: 'b',
	0x09: 't',
	0x0a: 'n',
	0x0b: 'v',
	0x0c: 'f',
	0x0d: 'r',
}

var immSigned = map[ImmType]bool{
	ImmSint:       true,
	ImmCodeOffset: true,
}

func none() ImmMeta                      { return ImmMeta{ImmNone, false, 0} }
func required(t ImmType) ImmMeta         { return ImmMeta{t, true, 0} }
func optional(t ImmType, b byte) ImmMeta { return ImmMeta{t, false, b} }

var opMeta = []OpMeta{
	OpMeta{
		Code: OpNOP,
		Imm0: none(),
		Imm1: none(),
		Imm2: none(),
		Name: "NOP",
	},
	OpMeta{
		Code: OpCHOICE,
		Imm0: required(ImmCodeOffset),
		Imm1: none(),
		Imm2: none(),
		Name: "CHOICE",
	},
	OpMeta{
		Code: OpCOMMIT,
		Imm0: required(ImmCodeOffset),
		Imm1: none(),
		Imm2: none(),
		Name: "COMMIT",
	},
	OpMeta{
		Code: OpFAIL,
		Imm0: none(),
		Imm1: none(),
		Imm2: none(),
		Name: "FAIL",
	},
	OpMeta{
		Code: OpANYB,
		Imm0: optional(ImmCount, 1),
		Imm1: none(),
		Imm2: none(),
		Name: "ANYB",
	},
	OpMeta{
		Code: OpSAMEB,
		Imm0: required(ImmByte),
		Imm1: optional(ImmCount, 1),
		Imm2: none(),
		Name: "SAMEB",
	},
	OpMeta{
		Code: OpLITB,
		Imm0: required(ImmLiteralIdx),
		Imm1: none(),
		Imm2: none(),
		Name: "LITB",
	},
	OpMeta{
		Code: OpMATCHB,
		Imm0: required(ImmMatcherIdx),
		Imm1: optional(ImmCount, 1),
		Imm2: none(),
		Name: "MATCHB",
	},
	OpMeta{
		Code: OpJMP,
		Imm0: required(ImmCodeOffset),
		Imm1: none(),
		Imm2: none(),
		Name: "JMP",
	},
	OpMeta{
		Code: OpCALL,
		Imm0: required(ImmCodeOffset),
		Imm1: none(),
		Imm2: none(),
		Name: "CALL",
	},
	OpMeta{
		Code: OpRET,
		Imm0: none(),
		Imm1: none(),
		Imm2: none(),
		Name: "RET",
	},
	OpMeta{
		Code: OpTANYB,
		Imm0: required(ImmCodeOffset),
		Imm1: optional(ImmCount, 1),
		Imm2: none(),
		Name: "TANYB",
	},
	OpMeta{
		Code: OpTSAMEB,
		Imm0: required(ImmCodeOffset),
		Imm1: required(ImmByte),
		Imm2: optional(ImmCount, 1),
		Name: "TSAMEB",
	},
	OpMeta{
		Code: OpTLITB,
		Imm0: required(ImmCodeOffset),
		Imm1: required(ImmLiteralIdx),
		Imm2: none(),
		Name: "TLITB",
	},
	OpMeta{
		Code: OpTMATCHB,
		Imm0: required(ImmCodeOffset),
		Imm1: required(ImmMatcherIdx),
		Imm2: optional(ImmCount, 1),
		Name: "TMATCHB",
	},
	OpMeta{
		Code: OpPCOMMIT,
		Imm0: required(ImmCodeOffset),
		Imm1: none(),
		Imm2: none(),
		Name: "PCOMMIT",
	},
	OpMeta{
		Code: OpBCOMMIT,
		Imm0: required(ImmCodeOffset),
		Imm1: none(),
		Imm2: none(),
		Name: "BCOMMIT",
	},
	OpMeta{
		Code: OpSPANB,
		Imm0: required(ImmMatcherIdx),
		Imm1: none(),
		Imm2: none(),
		Name: "SPANB",
	},
	OpMeta{
		Code: OpFAIL2X,
		Imm0: none(),
		Imm1: none(),
		Imm2: none(),
		Name: "FAIL2X",
	},
	OpMeta{
		Code: OpRWNDB,
		Imm0: required(ImmCount),
		Imm1: none(),
		Imm2: none(),
		Name: "RWNDB",
	},
	OpMeta{
		Code: OpFCAP,
		Imm0: required(ImmCaptureIdx),
		Imm1: required(ImmCount),
		Imm2: none(),
		Name: "FCAP",
	},
	OpMeta{
		Code: OpBCAP,
		Imm0: required(ImmCaptureIdx),
		Imm1: none(),
		Imm2: none(),
		Name: "BCAP",
	},
	OpMeta{
		Code: OpECAP,
		Imm0: required(ImmCaptureIdx),
		Imm1: none(),
		Imm2: none(),
		Name: "ECAP",
	},
	OpMeta{
		Code: OpGIVEUP,
		Imm0: none(),
		Imm1: none(),
		Imm2: none(),
		Name: "GIVEUP",
	},
	OpMeta{
		Code: OpEND,
		Imm0: none(),
		Imm1: none(),
		Imm2: none(),
		Name: "END",
	},
}

func init() {
	assert(sort.IsSorted(byCode(opMeta)), "IsSorted(byCode(opMeta))")
}
