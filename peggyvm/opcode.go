package peggyvm

import (
	"fmt"
	"sort"
)

// OpCode is an enum that identifies which instruction to perform.
type OpCode uint8

const (
	OpNOP    OpCode = 0x00
	OpCHOICE OpCode = 0x01
	OpCOMMIT OpCode = 0x02
	OpFAIL   OpCode = 0x03
	OpANYB   OpCode = 0x04
	OpSAMEB  OpCode = 0x05
	OpLITB   OpCode = 0x06
	OpMATCHB OpCode = 0x07

	// OpCodes above this line may be encoded as one-byte instructions.
	// --------------------
	// OpCodes below this line must use two-byte instructions.

	OpJMP OpCode = 0x08

	// 0x09 RESERVED

	OpCALL    OpCode = 0x0a
	OpRET     OpCode = 0x0b
	OpTANYB   OpCode = 0x0c
	OpTSAMEB  OpCode = 0x0d
	OpTLITB   OpCode = 0x0e
	OpTMATCHB OpCode = 0x0f
	OpPCOMMIT OpCode = 0x10
	OpBCOMMIT OpCode = 0x11
	OpSPANB   OpCode = 0x12
	OpFAIL2X  OpCode = 0x13
	OpRWNDB   OpCode = 0x14
	OpFCAP    OpCode = 0x15
	OpBCAP    OpCode = 0x16
	OpECAP    OpCode = 0x17

	// 0x18 .. 0x3d RESERVED

	OpGIVEUP OpCode = 0x3e
	OpEND    OpCode = 0x3f
)

func (c OpCode) Meta() *OpMeta {
	i := sort.Search(len(opMeta), func(i int) bool {
		return opMeta[i].Code >= c
	})
	if i < len(opMeta) && opMeta[i].Code == c {
		return &opMeta[i]
	}
	return &OpMeta{
		Code:    c,
		Illegal: true,
		Name:    fmt.Sprintf("ILLEGAL#%02x", byte(c)),
		Imm0:    optional(ImmUint, 0),
		Imm1:    optional(ImmUint, 0),
		Imm2:    optional(ImmUint, 0),
	}
}

func (c OpCode) String() string {
	return c.Meta().Name
}

// ImmType is an enum that describes how an immediate slot is used.
type ImmType uint8

const (
	// ImmNone says the slot is never used.
	ImmNone ImmType = iota

	// ImmUint says the slot holds an unsigned integer.
	ImmUint

	// ImmSint says the slot holds a *signed* integer.
	ImmSint

	// ImmByte says the slot holds a byte value (unsigned).
	ImmByte

	// ImmRune says the slot holds a Unicode rune value (unsigned).
	ImmRune

	// ImmCount says the slot holds an unsigned count.
	ImmCount

	// ImmCodeOffset says the slot holds a *signed* XP offset, relative to
	// the start of the *following* instruction.
	ImmCodeOffset

	// ImmLiteralIdx says the slot holds an unsigned literal index.
	ImmLiteralIdx

	// ImmMatcherIdx says the slot holds an unsigned ByteMatcher index.
	ImmMatcherIdx

	// ImmCaptureIdx says the slot holds an unsigned capture index.
	ImmCaptureIdx
)

func (t ImmType) Signed() bool {
	return immSigned[t]
}

// ImmMeta represents metadata about one of an OpCode's immediate slots.
type ImmMeta struct {
	Type          ImmType
	Required      bool
	PackedDefault byte
}

func (m ImmMeta) Default() uint64 {
	b := m.PackedDefault
	v := uint64(b)
	if m.Type.Signed() && (b&0x80) == 0x80 {
		v |= ^uint64(0xff)
	}
	return v
}

func (m ImmMeta) IsPresent(v uint64) bool {
	if m.Type == ImmNone {
		return false
	}
	if m.Required {
		return true
	}
	return m.Default() != v
}

func (m ImmMeta) Decode(data []byte) (value uint64, err error) {
	value = m.Default()

	if len(data) == 0 {
		if m.Type != ImmNone && m.Required {
			err = ErrMissingImmediate
		}
		return
	}
	if m.Type == ImmNone {
		err = ErrUnexpectedImmediate
		return
	}

	for i, b := range data {
		value |= uint64(b) << (uint(i) * 8)
	}
	b := data[len(data)-1]
	if m.Type.Signed() && (b&0x80) == 0x80 {
		for i := len(data); i < 8; i++ {
			value |= uint64(0xff) << (uint(i) * 8)
		}
	}
	return
}

func (m ImmMeta) Encode(v uint64) []byte {
	if !m.IsPresent(v) {
		return nil
	}

	var raw [8]byte
	raw[0] = byte(v)
	raw[1] = byte(v >> 8)
	raw[2] = byte(v >> 16)
	raw[3] = byte(v >> 24)
	raw[4] = byte(v >> 32)
	raw[5] = byte(v >> 40)
	raw[6] = byte(v >> 48)
	raw[7] = byte(v >> 56)

	x := byte(0x00)
	f := func(b byte) bool { return true }
	if m.Type.Signed() {
		y := byte(0x00)
		if (v & highbit) == highbit {
			x = 0xff
			y = 0x80
		}
		f = func(b byte) bool { return (b & 0x80) == y }
	}

	n := 8
	if raw[7] == x && raw[6] == x && raw[5] == x && raw[4] == x && f(raw[3]) {
		n = 4
		if raw[3] == x && raw[2] == x && f(raw[1]) {
			n = 2
			if raw[1] == x && f(raw[0]) {
				n = 1
			}
		}
	}

	return raw[0:n]
}

// OpMeta represents metadata about an OpCode.
type OpMeta struct {
	// Code holds the OpCode which this OpMeta is about.
	Code OpCode

	// Illegal is true iff this OpMeta contains synthesized, hypothetical
	// metadata about an invalid or reserved opcode.
	Illegal bool

	// Imm0, Imm1, and Imm2 hold information about the immediate slots.
	Imm0 ImmMeta
	Imm1 ImmMeta
	Imm2 ImmMeta

	// Name is the ASCII mnemonic for this opcode.
	Name string
}
