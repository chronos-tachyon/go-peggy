package peggyvm

import (
	"errors"
	"fmt"
)

var (
	ErrUnknownOpcode       = errors.New("invalid instruction: unknown opcode")
	ErrBadImmediateLen     = errors.New("invalid instruction: failed to decode length of immediate")
	ErrMissingImmediate    = errors.New("invalid instruction: missing immediate where one was expected")
	ErrUnexpectedImmediate = errors.New("invalid instruction: found immediate where none was expected")
	ErrExecutionHalted     = errors.New("execution halted")
)

type DisassembleError struct {
	Err error
	XP  uint64
}

func (e *DisassembleError) Error() string {
	return fmt.Sprintf("github.com/chronos-tachyon/peggy/peggyvm: disassembly error @ XP %d: %v", e.XP, e.Err)
}

type RuntimeError struct {
	Err error
	XP  uint64
	DP  uint64
}
