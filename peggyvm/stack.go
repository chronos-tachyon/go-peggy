package peggyvm

// Frame is a single frame on the call stack.
type Frame struct {
	// IsChoice is true iff this is a CHOICE/FAIL frame, or false iff this
	// is a CALL/RET frame.
	IsChoice bool

	// DP is the value of DP to use if the frame is restored.
	// (This field is only meaningful for CHOICE/FAIL frames.)
	DP uint64

	// XP is the value of XP to use if the frame is restored.
	// (This field is meaningful for both CALL/RET and CHOICE/FAIL frames.)
	XP uint64

	// KS is the value of KS to use if the frame is restored.
	// (This field is only meaningful for CHOICE/FAIL frames.)
	KS []Assignment
}
