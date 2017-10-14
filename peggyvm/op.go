package peggyvm

import (
	"bytes"
	"fmt"
	"io"
)

// Op is a single PEG instruction, decoded from raw bytecode.
type Op struct {
	// XP is the code address of the start of the instruction.
	XP uint64

	// Imm0, Imm1, and Imm2 are the instruction's immediates.
	Imm0 uint64
	Imm1 uint64
	Imm2 uint64

	// Meta is an *optional* pointer to the metadata about this
	// instruction's opcode.  If this is nil, use Code.Meta() instead.
	Meta *OpMeta

	// Code is this instruction's opcode.
	Code OpCode

	// Len is the actual encoded length of this instruction's bytecode.
	// Decoding of the next instruction will begin at XP+Len.
	Len uint
}

// String provides a programmer-friendly debugging string for the Op.
func (op *Op) String() string {
	var buf bytes.Buffer
	first := true

	f := func(m ImmMeta, v uint64) {
		if m.IsPresent(v) {
			if !first {
				buf.WriteByte(',')
			}
			fmt.Fprintf(&buf, "%d", v)
			first = false
		}
	}

	meta := op.Meta
	if meta == nil {
		meta = op.Code.Meta()
	}
	buf.WriteString(meta.Name)
	buf.WriteByte('<')
	f(meta.Imm0, op.Imm0)
	f(meta.Imm1, op.Imm1)
	f(meta.Imm2, op.Imm2)
	buf.WriteByte('>')
	return buf.String()
}

// Decode attempts to decode an instruction from the provided bytecode stream
// at the provided code address. Overwrites this Op's existing data.
func (op *Op) Decode(stream []byte, xp uint64) error {
	op.XP = xp
	op.Imm0 = 0
	op.Imm1 = 0
	op.Imm2 = 0
	op.Meta = nil
	op.Code = OpNOP
	op.Len = 1

	if xp >= uint64(len(stream)) {
		return io.EOF
	}

	byte0 := stream[xp]
	byte1 := byte(0xaa)
	hasByte1 := false
	nxp := xp + 1
	if nxp < uint64(len(stream)) {
		byte1 = stream[nxp]
		hasByte1 = true
		nxp += 1
	}

	var a, b, c, d byte
	if (byte0 & 0x80) == 0x80 {
		if !hasByte1 {
			return &DisassembleError{
				Err: io.ErrUnexpectedEOF,
				XP:  xp,
			}
		}
		op.Len = 2
		a = ((byte0 & 0x7e) >> 1)
		b = ((byte0 & 0x01) << 2) | ((byte1 & 0xc0) >> 6)
		c = ((byte1 & 0x38) >> 3)
		d = (byte1 & 0x07)
	} else {
		a = ((byte0 & 0x70) >> 4)
		b = ((byte0 & 0x0c) >> 2)
		c = (byte0 & 0x03)
		d = 0
	}

	len0, ok0 := ImmLengthDecode(b)
	len1, ok1 := ImmLengthDecode(c)
	len2, ok2 := ImmLengthDecode(d)

	if !ok0 || !ok1 || !ok2 {
		return &DisassembleError{
			Err: ErrBadImmediateLen,
			XP:  xp,
		}
	}

	i := xp + uint64(op.Len)
	j := i + uint64(len0)
	k := j + uint64(len1)
	l := k + uint64(len2)
	op.Len += len0 + len1 + len2
	if l > uint64(len(stream)) {
		return &DisassembleError{
			Err: io.ErrUnexpectedEOF,
			XP:  xp,
		}
	}

	meta := OpCode(a).Meta()

	imm0, err0 := meta.Imm0.Decode(stream[i:j])
	imm1, err1 := meta.Imm1.Decode(stream[j:k])
	imm2, err2 := meta.Imm2.Decode(stream[k:l])

	op.Meta = meta
	op.Code = meta.Code
	op.Imm0 = imm0
	op.Imm1 = imm1
	op.Imm2 = imm2

	var err error
	switch {
	case meta.Illegal:
		err = &DisassembleError{
			Err: ErrUnknownOpcode,
			XP:  xp,
		}

	case err0 != nil:
		err = &DisassembleError{
			Err: err0,
			XP:  xp,
		}

	case err1 != nil:
		err = &DisassembleError{
			Err: err1,
			XP:  xp,
		}

	case err2 != nil:
		err = &DisassembleError{
			Err: err2,
			XP:  xp,
		}
	}
	return err
}
