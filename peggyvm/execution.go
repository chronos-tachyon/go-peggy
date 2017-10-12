package peggyvm

import (
	"io"

	"github.com/chronos-tachyon/go-peggy/byteset"
)

type RunState uint8

const (
	RunningState RunState = iota
	SuccessState
	FailureState
	ErrorState
)

// Execution is the context of a match-in-progress.
type Execution struct {
	// P is the program to run.
	P *Program

	// I is the input bytestring on which the match is executing.
	I []byte

	// DP (Data Pointer) is the index into I of the current byte.
	DP uint64

	// XP (eXecution Pointer) is the index into P.Bytes of the Op to decode
	// and execute *next*, i.e. after the current Op completes.
	XP uint64

	// KS is the current stack of capture assignments.
	//
	// - KS is append-only. It grows when one of the FCAP, BCAP, or ECAP
	//   instructions executes, pushing one (BCAP/ECAP) or two (FCAP) items
	//   on the stack. While KS is never popped, it may be restored to an
	//   earlier (shorter) version by the FAIL or BCOMMIT instructions.
	//
	// - For multiple assignments to the same (Index, IsEnd) tuple, the
	//   assignment closest to the top of the stack takes precedence.
	//
	KS []Assignment

	// CS is the current stack of CALL/RET and CHOICE/FAIL frames.
	//
	// - CALL pushes a CALL/RET frame.
	//
	// - RET pops a CALL/RET frame and restores it. It is an error to RET
	//   when a CHOICE/FAIL frame is pending.
	//
	// - CHOICE pushes a CHOICE/FAIL frame.
	//
	// - COMMIT pops a CHOICE/FAIL frame. It is an error to COMMIT when a
	//   CALL/RET frame is pending.
	//
	// - FAIL pops zero or more CALL/RET frames, ignoring them, then pops
	//   at most one CHOICE/FAIL frame and restores it. If no CHOICE/FAIL
	//   frame is pending, then FAIL is equivalent to GIVEUP.
	//
	// - Many other instructions have behaviors similar to COMMIT, FAIL, or
	//   a combination of the two.
	//
	CS []Frame

	R RunState
}

func (x *Execution) popCS() (Frame, bool) {
	if len(x.CS) == 0 {
		return Frame{}, false
	}
	i := len(x.CS) - 1
	fr := x.CS[i]
	x.CS = x.CS[:i]
	return fr, true
}

func (x *Execution) availableBytes() uint64 {
	assert(x.DP <= uint64(len(x.I)), "DP out of range")
	return uint64(len(x.I)) - x.DP
}

func (x *Execution) end() {
	x.R = SuccessState
}

func (x *Execution) giveUp() {
	x.R = FailureState
	x.KS = nil
}

func (x *Execution) fail() {
	for {
		fr, ok := x.popCS()
		if !ok {
			x.giveUp()
			return
		}
		if fr.IsChoice {
			x.DP = fr.DP
			x.XP = fr.XP
			x.KS = fr.KS
			return
		}
	}
}

func (x *Execution) matchN(m byteset.Matcher, n uint64) bool {
	if x.availableBytes() < n {
		return false
	}
	for i := uint64(0); i < n; i++ {
		if !m.Match(x.I[x.DP+i]) {
			return false
		}
	}
	return true
}

func (x *Execution) matchLit(l []byte) (uint64, bool) {
	n := uint64(len(l))
	if x.availableBytes() < n {
		return 0, false
	}
	for i := uint64(0); i < n; i++ {
		if x.I[x.DP+i] != l[i] {
			return 0, false
		}
	}
	return n, true
}

func (x *Execution) Step() error {
	if x.R != RunningState {
		return ErrExecutionHalted
	}

	var op Op
	err := op.Decode(x.P.Bytes, x.XP)
	if err == io.EOF {
		x.end()
		return nil
	}
	if err != nil {
		x.R = ErrorState
		x.KS = nil
		return err
	}

	x.XP += uint64(op.Len)
	switch op.Code {
	case OpNOP:
		// pass

	case OpCHOICE:
		x.CS = append(x.CS, Frame{
			IsChoice: true,
			DP:       x.DP,
			XP:       addOffset(x.XP, u2s(op.Imm0)),
			KS:       x.KS,
		})

	case OpCOMMIT:
		fr, ok := x.popCS()
		assert(ok, "COMMIT on empty stack")
		assert(fr.IsChoice, "COMMIT on CALL/RET frame")
		x.XP = addOffset(x.XP, u2s(op.Imm0))

	case OpFAIL:
		x.fail()

	case OpANYB:
		if x.availableBytes() >= op.Imm0 {
			x.DP += op.Imm0
		} else {
			x.fail()
		}

	case OpSAMEB:
		if x.matchN(byteset.Exactly(byte(op.Imm0)), op.Imm1) {
			x.DP += op.Imm1
		} else {
			x.fail()
		}

	case OpLITB:
		assert(op.Imm0 < uint64(len(x.P.Literals)), "LITB literal index out of range")
		if n, good := x.matchLit(x.P.Literals[op.Imm0]); good {
			x.DP += n
		} else {
			x.fail()
		}

	case OpMATCHB:
		assert(op.Imm0 < uint64(len(x.P.ByteSets)), "MATCHB byteset index out of range")
		if x.matchN(x.P.ByteSets[op.Imm0], op.Imm1) {
			x.DP += op.Imm1
		} else {
			x.fail()
		}

	case OpJMP:
		x.XP = addOffset(x.XP, u2s(op.Imm0))

	case OpCALL:
		x.CS = append(x.CS, Frame{
			IsChoice: false,
			XP:       x.XP,
		})
		x.XP = addOffset(x.XP, u2s(op.Imm0))

	case OpRET:
		fr, ok := x.popCS()
		assert(ok, "RET on empty stack")
		assert(!fr.IsChoice, "RET on CHOICE/FAIL frame")
		x.XP = fr.XP

	case OpTANYB:
		if x.availableBytes() >= op.Imm1 {
			x.DP += op.Imm1
		} else {
			x.XP = addOffset(x.XP, u2s(op.Imm0))
		}

	case OpTSAMEB:
		if x.matchN(byteset.Exactly(byte(op.Imm1)), op.Imm2) {
			x.DP += op.Imm2
		} else {
			x.XP = addOffset(x.XP, u2s(op.Imm0))
		}

	case OpTLITB:
		assert(op.Imm1 < uint64(len(x.P.Literals)), "TLITB literal index out of range")
		if n, good := x.matchLit(x.P.Literals[op.Imm1]); good {
			x.DP += n
		} else {
			x.XP = addOffset(x.XP, u2s(op.Imm0))
		}

	case OpTMATCHB:
		assert(op.Imm1 < uint64(len(x.P.ByteSets)), "TMATCHB byteset index out of range")
		if x.matchN(x.P.ByteSets[op.Imm1], op.Imm2) {
			x.DP += op.Imm2
		} else {
			x.XP = addOffset(x.XP, u2s(op.Imm0))
		}

	case OpPCOMMIT:
		fr, ok := x.popCS()
		assert(ok, "PCOMMIT on empty stack")
		assert(fr.IsChoice, "PCOMMIT on CALL/RET frame")
		fr.DP = x.DP
		fr.XP = addOffset(x.XP, u2s(op.Imm0))
		fr.KS = x.KS
		x.CS = append(x.CS, fr)

	case OpBCOMMIT:
		fr, ok := x.popCS()
		assert(ok, "BCOMMIT on empty stack")
		assert(fr.IsChoice, "BCOMMIT on CALL/RET frame")
		x.DP = fr.DP
		x.KS = fr.KS
		x.XP = addOffset(x.XP, u2s(op.Imm0))

	case OpSPANB:
		assert(op.Imm0 < uint64(len(x.P.ByteSets)), "SPANB byteset index out of range")
		for m, n := x.P.ByteSets[op.Imm0], uint64(len(x.I)); x.DP < n && m.Match(x.I[x.DP]); x.DP += 1 {
			// pass
		}

	case OpFAIL2X:
		fr, ok := x.popCS()
		assert(ok, "FAIL2X on empty stack")
		assert(fr.IsChoice, "FAIL2X on CALL/RET frame")
		x.fail()

	case OpRWNDB:
		assert(x.DP >= op.Imm0, "RWNDB byte count out of range")
		x.DP -= op.Imm0

	case OpFCAP:
		assert(x.DP >= op.Imm1, "FCAP byte count out of range")
		x.KS = append(x.KS, Assignment{
			Index: op.Imm0,
			IsEnd: false,
			DP:    x.DP - op.Imm1,
		})
		x.KS = append(x.KS, Assignment{
			Index: op.Imm0,
			IsEnd: true,
			DP:    x.DP,
		})

	case OpBCAP:
		x.KS = append(x.KS, Assignment{
			Index: op.Imm0,
			IsEnd: false,
			DP:    x.DP,
		})

	case OpECAP:
		x.KS = append(x.KS, Assignment{
			Index: op.Imm0,
			IsEnd: true,
			DP:    x.DP,
		})

	case OpGIVEUP:
		x.giveUp()

	case OpEND:
		x.end()
	}
	return nil
}

func (x *Execution) Run() error {
	for x.R == RunningState {
		err := x.Step()
		if err != nil {
			return err
		}
	}
	return nil
}
