// Package peggyvm implements a virtual machine for Parsing Expression Grammars.
//
//
// The VM uses the following instruction encoding for its bytecode:
//
// ONE BYTE INSTRUCTION PLUS ZERO TO TWO IMMEDIATES:
//
//   [ 0aaa | bbcc ] ...imm0 ...imm1
//
//   aaa = Opcode
//    bb = Encoded size of imm0
//    cc = Encoded size of imm1
//
//   +----------------+
//   | Size encoding  |
//   +-----+----------+
//   |  00 | absent   |
//   |  01 | 8 bits   |
//   |  10 | 16 bits  |
//   |  11 | 32 bits  |
//   +-----+----------+
//
// TWO BYTE INSTRUCTION PLUS ZERO TO THREE IMMEDIATES:
//
//   [ 1aaa | aaab ] [ bbcc | cddd ] ...imm0 ...imm1 ...imm2
//
//   aaaaaa = Opcode
//      bbb = Encoded size of imm0
//      ccc = Encoded size of imm1
//      ddd = Encoded size of imm2
//
//   +----------------+
//   | Size encoding  |
//   +-----+----------+
//   | 000 | absent   |
//   | 001 | 8 bits   |
//   | 010 | 16 bits  |
//   | 011 | 32 bits  |
//   | 100 | 64 bits  |
//   | 101 | reserved |
//   | 110 | reserved |
//   | 111 | reserved |
//   +-----+----------+
//
// In the above information, the following statements hold:
//
// • Leftmost bits are most significant.
//
// • Immediates are stored in little endian byte order.
//
// • Signed immediates are stored in 2's complement form.
//
// The one-byte encoding is preferred if possible. However, the two-byte
// encoding is required if any of the following are true:
//
// • The opcode is outside the range [0..7]
//
// • All three immediates are being provided to the instruction
//
// • Any of the immediates cannot be represented in 32 bits
//
//
// The opcodes are organized in the following fashion:
//
//   +------+---------+---------+---------+---------+
//   |      | 00      | 01      | 10      | 11      |
//   +------+---------+---------+---------+---------+
//   | 0000 | NOP     | CHOICE  | COMMIT  | FAIL    |
//   | 0001 | ANYB    | SAMEB   | LITB    | MATCHB  |
//   | 0010 | JMP     | -       | CALL    | RET     |
//   | 0011 | TANYB   | TSAMEB  | TLITB   | TMATCHB |
//   +------+---------+---------+---------+---------+
//   | 0100 | PCOMMIT | BCOMMIT | SPANB   | FAIL2X  |
//   | 0101 | RWNDB   | FCAP    | BCAP    | ECAP    |
//   | 0110 | -       | -       | -       | -       |
//   | 0111 | -       | -       | -       | -       |
//   +------+---------+---------+---------+---------+
//   | 1000 | -       | -       | -       | -       |
//   | 1001 | -       | -       | -       | -       |
//   | 1010 | -       | -       | -       | -       |
//   | 1011 | -       | -       | -       | -       |
//   +------+---------+---------+---------+---------+
//   | 1100 | -       | -       | -       | -       |
//   | 1101 | -       | -       | -       | -       |
//   | 1110 | -       | -       | -       | -       |
//   | 1111 | -       | -       | GIVEUP  | END     |
//   +------+---------+---------+---------+---------+
//
//   (Left: bits 5-4-3-2; top: bits 1-0.)
//
// The actual opcodes now follow, with their behaviors explained both with
// prose and with Go-like pseudocode.
//
// • NOP (0x00)
//
//   NOP
//
// Short for "No Operation". Does nothing but take up space.
//
// • CHOICE (0x01)
//
//   CHOICE imm0
//   imm0: required ImmCodeOffset (signed)
//
//   altDP := exec.DP
//   altXP := exec.XP + imm0
//   altKS := exec.KS
//   exec.CS.push({
//     IsChoice: true,
//     DP:       altDP,
//     XP:       altXP,
//     KS:       altKS,
//   })
//
// Sets up an alternative parse: if the current parse fails, the parse state
// will be rewound and execution will transfer to imm0.
//
// • COMMIT (0x02)
//
//   COMMIT imm0
//   imm0: required ImmCodeOffset (signed)
//
//   frame, ok := exec.CS.pop()
//   assert(ok && frame.IsChoice)
//   exec.XP += imm0
//
// Commits to the current parse & jumps to imm0.
//
// • FAIL (0x03)
//
//   FAIL
//
//   func topmostChoice() (Frame, bool) {
//     for !exec.CS.isEmpty() {
//       frame := exec.CS.pop()
//       if frame.IsChoice { return frame, true }
//     }
//     return Frame{}, false
//   }
//
//   frame, ok := topmostChoice()
//   if ok {
//     exec.DP = frame.DP
//     exec.XP = frame.XP
//     exec.KS = frame.KS
//   } else {
//     giveUp()
//   }
//
// Fails the match, backtracking the data stream and capture stack and jumping
// to the saved imm0 of the last CHOICE.
//
// • ANYB (0x04)
//
//   ANYB [imm0]
//   imm0: optional ImmCount (default: 1)
//
//   func availableBytes() uint64 {
//     return exec.I.Len() - exec.DP
//   }
//
//   func isMatchingSequence(m byteset.Matcher, n int) bool {
//     if n > availableBytes() { return false }
//     for i := 0; i < n; i++ {
//       b := exec.I[exec.DP + i]
//       if !m.MatchByte(b) { return false }
//     }
//     return true
//   }
//
//   matcher := byteset.All()
//   good := isMatchingSequence(matcher, imm0)
//   if good {
//     exec.DP += imm0
//   } else {
//     fail()
//   }
//
// Matches imm0 bytes, each of which may have any value. Fails if fewer than
// imm0 bytes of data remain.
//
// • SAMEB (0x05)
//
//   SAMEB imm0[, imm1]
//   imm0: required ImmByte
//   imm1: optional ImmCount (default: 1)
//
//   matcher := byteset.Exactly(imm0)
//   good := isMatchingSequence(matcher, imm1)
//   if good {
//     exec.DP += imm1
//   } else {
//     fail()
//   }
//
// Matches imm1 bytes, each of which has the exact value imm0. Fails if any of
// the next imm1 bytes has a value other than imm0, or if fewer than imm1 bytes
// of data remain.
//
// • LITB (0x06)
//
//   LITB imm0
//   imm0: required ImmLiteralIdx
//
//   func isMatchingLiteral(literal []byte) bool {
//     if availableBytes() < len(literal) { return false }
//     for i, b1 := range literal {
//       b2 := exec.I[exec.DP + i]
//       if b1 != b2 { return false }
//     }
//     return true
//   }
//
//   literal := exec.P.Literals[imm0]
//   good := isMatchingLiteral(literal)
//   if good {
//     exec.DP += len(literal)
//   } else {
//     fail()
//   }
//
// Matches the literal bytestring with index imm0. Fails if, for any byte index
// i ∈ [0 .. |literal|-1], the i-th byte of the data doesn't equal the i-th
// byte of the literal, or if fewer than |literal| bytes of data remain.
//
// • MATCHB (0x07)
//
//   MATCHB imm0[, imm1]
//   imm0: required ImmMatcherIdx
//   imm1: optional ImmCount (default: 1)
//
//   matcher := exec.P.ByteSets[imm0]
//   good := isMatchingSequence(matcher, imm1)
//   if good {
//     exec.DP += imm1
//   } else {
//     fail()
//   }
//
// Matches imm1 bytes using the byteset.Matcher with index imm0. Fails if the
// byteset.Matcher fails to match any of the next imm1 bytes, or if fewer than imm1
// bytes of data remain.
//
// • JMP (0x08)
//
//   JMP imm0
//   imm0: required ImmCodeOffset (signed)
//
//   exec.XP += imm0
//
// Unconditionally jumps to imm0.
//
// • CALL (0x0a)
//
//   CALL imm0
//   imm0: required ImmCodeOffset (signed)
//
//   exec.CS.push({
//     IsChoice: false,
//     DP:       0,
//     XP:       exec.XP,
//     KS:       nil,
//   })
//   exec.XP += imm0
//
// Sets up a CALL/RET frame & jumps to imm0.
//
// • RET (0x0b)
//
//   RET
//
//   frame, ok := exec.CS.pop()
//   assert(ok && !frame.IsChoice)
//   exec.XP = frame.XP
//
// Pops a CALL/RET frame, jumping back to the instruction that directly
// followed the invoking CALL.
//
// • TANYB (0x0c)
//
//   TANYB imm0[, imm1]
//   imm0: required ImmCodeOffset (signed)
//   imm1: optional ImmCount (default: 1)
//
//   matcher := byteset.All()
//   good := isMatchingSequence(matcher, imm1)
//   if good {
//     exec.DP += imm1
//   } else {
//     exec.XP += imm0
//   }
//
// Matches imm1 bytes, each of which may have any value. Jumps to imm0 if fewer
// than imm0 bytes of data remain.
//
// • TSAMEB (0x0d)
//
//   TSAMEB imm0, imm1[, imm2]
//   imm0: required ImmCodeOffset (signed)
//   imm1: required ImmByte
//   imm2: optional ImmCount (default: 1)
//
//   matcher := byteset.Exactly(imm1)
//   good := isMatchingSequence(matcher, imm2)
//   if good {
//     exec.DP += imm2
//   } else {
//     exec.XP += imm0
//   }
//
// Matches imm2 bytes, each of which has the exact value imm1. Jumps to imm0 if
// any of the next imm2 bytes has a value other than imm1, or if fewer than
// imm2 bytes of data remain.
//
// • TLITB (0x0e)
//
//   TLITB imm0, imm1
//   imm0: required ImmCodeOffset (signed)
//   imm1: required ImmLiteralIdx
//
//   literal := exec.P.Literals[imm1]
//   good := isMatchingLiteral(literal)
//   if good {
//     exec.DP += len(literal)
//   } else {
//     exec.XP += imm0
//   }
//
// Matches the literal bytestring with index imm1. Jumps to imm0 if, for any
// byte index i ∈ [0 .. |literal|-1], the i-th byte of the data doesn't equal
// the i-th byte of the literal, or if fewer than |literal| bytes of data
// remain.
//
// • TMATCHB (0x0f)
//
//   TMATCHB imm0, imm1[, imm2]
//   imm0: required ImmCodeOffset (signed)
//   imm1: required ImmMatcherIdx
//   imm2: optional ImmCount (default: 1)
//
//   matcher := exec.P.ByteSets[imm1]
//   good := isMatchingSequence(matcher, imm2)
//   if good {
//     exec.DP += imm2
//   } else {
//     exec.XP += imm0
//   }
//
// Matches imm2 bytes using the byteset.Matcher with index imm1. Jumps to imm0 if
// the byteset.Matcher fails to match any of the next imm2 bytes, or if fewer than
// imm2 bytes remain.
//
// • PCOMMIT (0x10)
//
//   PCOMMIT imm0
//   imm0: required ImmCodeOffset (signed)
//
//   frame, ok := exec.CS.pop()
//   assert(ok && frame.IsChoice)
//   frame.DP = exec.DP
//   frame.XP = exec.XP + imm0
//   frame.KS = exec.KS
//   exec.CS.push(frame)
//
// Updates the alternative parse already set up by a previous CHOICE:
// if the current parse fails, the parse state will now be rewound to
// the current state (not the state at the last CHOICE) and execution
// will transfer to PCOMMIT's imm0 (not to CHOICE's imm0).
//
// Used to efficiently implement greedy loops.
//
// • BCOMMIT (0x11)
//
//   BCOMMIT imm0
//   imm0: required ImmCodeOffset (signed)
//
//   frame, ok := exec.CS.pop()
//   assert(ok && frame.IsChoice)
//   exec.DP = frame.DP
//   exec.XP += imm0  // ignore frame.XP
//   exec.KS = frame.KS
//
// Backtracks the data stream and capture stack (like a FAIL), but
// jumps to BCOMMIT's imm0 (not the CHOICE's imm0).
//
// Used to efficiently implement positive lookahead assertions.
//
// • SPANB (0x12)
//
//   SPANB imm0
//   imm0: required ImmMatcherIdx
//
//   matcher := exec.P.ByteSets[imm0]
//   for availableBytes() >= 1 {
//     b := exec.I[exec.DP]
//     if !matcher.MatchByte(b) { break }
//     exec.DP += 1
//   }
//
// Greedily matches zero or more bytes using the byteset.Matcher with index
// imm0. Always succeeds.
//
// • FAIL2X (0x13)
//
//   FAIL2X
//
//   frame, ok := exec.CS.pop()
//   assert(ok && frame.IsChoice)
//   fail()
//
// Fails the match twice.
//
// Used to efficiently implement negative lookahead assertions.
//
// • RWNDB (0x14)
//
//   RWNDB imm0
//   imm0: required ImmCount
//
//   assert(exec.DP >= imm0)
//   exec.DP -= imm0
//
// Rewinds the data stream by imm0 bytes.
//
// • FCAP (0x15)
//
//   FCAP imm0, imm1
//   imm0: required ImmCaptureIdx
//   imm1: required ImmCount
//
//   assert(exec.DP >= imm1)
//   exec.KS.push({
//     Index: imm0,
//     IsEnd: false,
//     DP:    exec.DP - imm1,
//   })
//   exec.KS.push({
//     Index: imm0,
//     IsEnd: true,
//     DP:    exec.DP,
//   })
//
// Records that the capture with index imm0 now contains the last imm1 bytes.
//
// • BCAP (0x16)
//
//   BCAP imm0
//   imm0: required ImmCaptureIdx
//
//   exec.KS.push({
//     Index: imm0,
//     IsEnd: false,
//     DP:    exec.DP,
//   })
//
// Records that the capture with index imm0 begins at this data position.
//
// • ECAP (0x17)
//
//   ECAP imm0
//   imm0: required ImmCaptureIdx
//
//   exec.KS.push({
//     Index: imm0,
//     IsEnd: true,
//     DP:    exec.DP,
//   })
//
// Records that the capture with index imm0 ends at this data position.
//
// • GIVEUP (0x3e)
//
//   GIVEUP
//
// Unconditionally fails the outermost match, ignoring the stack.
//
// • END (0x3f)
//
//   END
//
// Unconditionally succeeds at the outermost match, ignoring the stack.
//
package peggyvm
