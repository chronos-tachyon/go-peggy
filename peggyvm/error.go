package peggyvm

import (
	"bytes"
	"errors"
	"fmt"
)

var (
	ErrUnknownOpcode       = errors.New("invalid instruction: unknown opcode")
	ErrBadImmediateLen     = errors.New("invalid instruction: failed to decode length of immediate")
	ErrMissingImmediate    = errors.New("invalid instruction: missing immediate where one was expected")
	ErrUnexpectedImmediate = errors.New("invalid instruction: found immediate where none was expected")
	ErrExecutionHalted     = errors.New("execution already halted")
	ErrEmptyStack          = errors.New("empty stack")
	ErrCallRetFrame        = errors.New("encountered CALL/RET stack frame")
	ErrChoiceFailFrame     = errors.New("encountered CHOICE/FAIL stack frame")
	ErrIndexRange          = errors.New("index out of range")
	ErrCountRange          = errors.New("count out of range")
)

// DisassembleError is an error encountered during the decoding of a compiled
// bytecode program. This typically means that corrupt or hostile bytecode is
// being run.
type DisassembleError struct {
	Err error
	XP  uint64
}

func (e *DisassembleError) Error() string {
	return fmt.Sprintf("github.com/chronos-tachyon/peggy/peggyvm: disassemble error @ XP %d: %v", e.XP, e.Err)
}

// RuntimeError is an error encountered during the execution of a compiled
// bytecode program. This typically means that there is a bug in the VM, or
// that corrupt or hostile bytecode is being run.
type RuntimeError struct {
	Err error
	XP  uint64
	DP  uint64
	Op  *Op
}

func (e *RuntimeError) Error() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "github.com/chronos-tachyon/peggy/peggyvm: runtime error @ XP %d DP %d: ", e.XP, e.DP)
	if e.Op != nil {
		meta := e.Op.Meta
		if meta == nil {
			meta = e.Op.Code.Meta()
		}
		buf.WriteString(meta.Name)
		buf.WriteString(": ")
	}
	buf.WriteString(e.Err.Error())
	return buf.String()
}
