package peggyvm

import (
	"bytes"
	"fmt"
)

// Result is the outcome of an Execution.
type Result struct {
	Success  bool
	Captures []Capture
}

// String provides a programmer-friendly debugging string for the Result.
func (r Result) String() string {
	var buf bytes.Buffer
	buf.WriteByte('{')
	fmt.Fprintf(&buf, "%v", r.Success)
	if r.Success {
		buf.WriteByte(' ')
		buf.WriteByte('[')
		first := true
		for i, c := range r.Captures {
			if !first {
				buf.WriteByte(' ')
			}
			fmt.Fprintf(&buf, "%d:%s", i, c)
			first = false
		}
		buf.WriteByte(']')
	}
	buf.WriteByte('}')
	return buf.String()
}
