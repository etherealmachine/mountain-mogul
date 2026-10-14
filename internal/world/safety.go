package world

// The resort's Safety score (notes/next/Avalanche Control.md): 0–1, shown
// beside the stars. Incidents knock it down and it recovers by the day;
// while it's low, fewer guests come and the stars can't rise above what
// it allows. Avalanches feed it now; lift failures and crowding later.
const (
	// SafetyBurial and SafetyPatrollerCaught are what a guest, and a
	// patroller, caught in an avalanche take off it.
	SafetyBurial          = float32(0.25)
	SafetyPatrollerCaught = float32(0.15)
	// SafetyRecoverPerDay is what it gets back each day.
	SafetyRecoverPerDay = float32(0.05)
	// SafetyCapFrom is the score under which it caps the stars.
	SafetyCapFrom = float32(0.8)
)

// SafetyIncident takes amount off the Safety score.
func (w *World) SafetyIncident(amount float32) {
	w.Safety = clamp01(w.Safety - amount)
}

// SafetyDemand is the share of guests who still come at the Safety score.
func (w *World) SafetyDemand() float32 { return 0.5 + 0.5*w.Safety }

// SafetyStarCap is the most stars the rating can show at the Safety
// score: 5 from SafetyCapFrom up, down to 1 at none.
func (w *World) SafetyStarCap() float32 {
	if w.Safety >= SafetyCapFrom {
		return 5
	}
	return 1 + 4*w.Safety/SafetyCapFrom
}
