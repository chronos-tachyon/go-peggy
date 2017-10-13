package peggyvm

import (
	"io"

	"github.com/chronos-tachyon/go-peggy/byteset"
)

// ExecutionState records information about whether an Execution has
// terminated, and why it was terminated if it was.
type ExecutionState uint8

const (
	// RunningState means the Execution has not terminated.
	RunningState ExecutionState = iota

	// SuccessState means the Execution has terminated normally with a
	// successful match of the input.
	SuccessState

	// FailureState means the Execution has terminated normally but was
	// unable to match the input.
	FailureState

	// FailureState means the Execution has terminated abnormally due to an
	// error in the program itself.
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

	R ExecutionState
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
	return uint64(len(x.I)) - x.DP
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

func (x *Execution) fail() {
	for {
		fr, ok := x.popCS()
		if !ok {
			x.R = FailureState
			x.KS = nil
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

// Step attempts to execute the next bytecode instruction.
func (x *Execution) Step() error {
	if x.R != RunningState {
		return ErrExecutionHalted
	}

	var op Op
	err := op.Decode(x.P.Bytes, x.XP)
	if err == io.EOF {
		x.R = SuccessState
		return nil
	}
	if err != nil {
		x.R = ErrorState
		x.KS = nil
		return err
	}

	rterr := func(err error) error {
		x.R = ErrorState
		x.KS = nil
		return &RuntimeError{
			Err: err,
			XP:  op.XP,
			DP:  x.DP,
			Op:  &op,
		}
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
		if !ok {
			return rterr(ErrEmptyStack)
		}
		if !fr.IsChoice {
			return rterr(ErrCallRetFrame)
		}
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
		if op.Imm0 >= uint64(len(x.P.Literals)) {
			return rterr(ErrIndexRange)
		}
		if n, good := x.matchLit(x.P.Literals[op.Imm0]); good {
			x.DP += n
		} else {
			x.fail()
		}

	case OpMATCHB:
		if op.Imm0 >= uint64(len(x.P.ByteSets)) {
			return rterr(ErrIndexRange)
		}
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
		if !ok {
			return rterr(ErrEmptyStack)
		}
		if !fr.IsChoice {
			return rterr(ErrChoiceFailFrame)
		}
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
		if op.Imm1 >= uint64(len(x.P.Literals)) {
			return rterr(ErrIndexRange)
		}
		if n, good := x.matchLit(x.P.Literals[op.Imm1]); good {
			x.DP += n
		} else {
			x.XP = addOffset(x.XP, u2s(op.Imm0))
		}

	case OpTMATCHB:
		if op.Imm1 >= uint64(len(x.P.ByteSets)) {
			return rterr(ErrIndexRange)
		}
		if x.matchN(x.P.ByteSets[op.Imm1], op.Imm2) {
			x.DP += op.Imm2
		} else {
			x.XP = addOffset(x.XP, u2s(op.Imm0))
		}

	case OpPCOMMIT:
		fr, ok := x.popCS()
		if !ok {
			return rterr(ErrEmptyStack)
		}
		if !fr.IsChoice {
			return rterr(ErrCallRetFrame)
		}
		fr.DP = x.DP
		fr.XP = addOffset(x.XP, u2s(op.Imm0))
		fr.KS = x.KS
		x.CS = append(x.CS, fr)

	case OpBCOMMIT:
		fr, ok := x.popCS()
		if !ok {
			return rterr(ErrEmptyStack)
		}
		if !fr.IsChoice {
			return rterr(ErrCallRetFrame)
		}
		x.DP = fr.DP
		x.KS = fr.KS
		x.XP = addOffset(x.XP, u2s(op.Imm0))

	case OpSPANB:
		if op.Imm0 >= uint64(len(x.P.ByteSets)) {
			return rterr(ErrIndexRange)
		}
		for m, n := x.P.ByteSets[op.Imm0], uint64(len(x.I)); x.DP < n && m.Match(x.I[x.DP]); x.DP += 1 {
			// pass
		}

	case OpFAIL2X:
		fr, ok := x.popCS()
		if !ok {
			return rterr(ErrEmptyStack)
		}
		if !fr.IsChoice {
			return rterr(ErrCallRetFrame)
		}
		x.fail()

	case OpRWNDB:
		if op.Imm0 > x.DP {
			return rterr(ErrCountRange)
		}
		x.DP -= op.Imm0

	case OpFCAP:
		if op.Imm0 >= uint64(len(x.P.Captures)) {
			return rterr(ErrIndexRange)
		}
		if op.Imm1 > x.DP {
			return rterr(ErrCountRange)
		}
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
		if op.Imm0 >= uint64(len(x.P.Captures)) {
			return rterr(ErrIndexRange)
		}
		x.KS = append(x.KS, Assignment{
			Index: op.Imm0,
			IsEnd: false,
			DP:    x.DP,
		})

	case OpECAP:
		if op.Imm0 >= uint64(len(x.P.Captures)) {
			return rterr(ErrIndexRange)
		}
		x.KS = append(x.KS, Assignment{
			Index: op.Imm0,
			IsEnd: true,
			DP:    x.DP,
		})

	case OpGIVEUP:
		x.R = FailureState
		x.KS = nil

	case OpEND:
		x.R = SuccessState
	}
	return nil
}

// Run attempts to execute the bytecode program to completion.
//
// WARNING: No time limits are enforced, and it's easy to write an infinite
//          loop. Think carefully before running untrusted bytecode.
//
func (x *Execution) Run() error {
	for x.R == RunningState {
		err := x.Step()
		if err != nil {
			return err
		}
	}
	return nil
}
