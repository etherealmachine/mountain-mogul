//go:build !demo

package save

import "mountain-mogul/internal/world"

// liftDataExtras is a heli lift's flight: HeliPhase and HeliProgress are
// only meaningful when Type == LiftHeli.
type liftDataExtras struct {
	HeliPhase    uint8   `json:"heli_phase,omitempty"`
	HeliProgress float32 `json:"heli_progress,omitempty"`
}

// saveHeli records heli lift l's flight in ld.
func saveHeli(l *world.Lift, ld *LiftData) {
	if l.HeliState != nil {
		ld.HeliPhase = uint8(l.HeliState.Phase)
		ld.HeliProgress = l.HeliState.Progress
	}
}

// loadHeli restores heli lift l's flight from ld.
func loadHeli(l *world.Lift, ld LiftData) {
	if l.HeliState != nil {
		l.HeliState.Phase = world.HeliPhase(ld.HeliPhase)
		l.HeliState.Progress = ld.HeliProgress
	}
}
