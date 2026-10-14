package world

// Wind holds and test runs (notes/next/Lift Operations.md). Each lift
// type has a limit for the wind at its top; past it the lift holds:
// riders on it come up, nobody boards. Once the wind has stayed under
// WindResumeShare of the limit for WindCalmHours, the lift runs empty on
// a test run for TestRunHours, guests free to queue, and then opens. The
// player can put a lift on a test run by hand (Lift.Testing).
const (
	WindResumeShare = float32(0.85)
	WindCalmHours   = 0.5
	TestRunHours    = 1.0 / 3
)

// WindLimit is the wind at the top, m/s (gusts included), a lift of type
// t holds in: chairs first, cabins later, the funitel and the 3S, built
// for it, last; a tow on the snow barely minds.
func (t LiftType) WindLimit() float32 {
	switch {
	case t == LiftFunitel:
		return 28
	case t == LiftTri:
		return 30
	case t.Tram():
		return 24
	case t == LiftGondola:
		return 21
	case t.Hangers():
		return 24
	case t.Surface():
		return 0 // the carpet and rope tow never hold
	case t.Detachable():
		return 18
	}
	return 16
}

// TestingNow reports whether lift l is on a test run at simTime: by hand,
// or after a wind hold. It runs empty; guests may queue but not board.
func (l *Lift) TestingNow(simTime float64) bool {
	return l.Testing || simTime < l.TestUntil
}
