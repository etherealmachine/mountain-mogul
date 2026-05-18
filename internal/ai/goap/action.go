package goap

import (
	"fmt"
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// Action is one step in a plan. Preconditions read the snapshot and the
// live world (lift queues, building positions); effects mutate a *copy*
// of the snapshot the planner expands during search. Costs are positive
// reals summed by A*; per-agent multipliers stay in the same shape so
// goalWeight / actionCost are the only two surfaces a designer touches.
type Action interface {
	Name() string
	Precondition(s *WorldSnapshot, w *world.World) bool
	Apply(s *WorldSnapshot, w *world.World)
	Cost(s *WorldSnapshot, w *world.World) float32
}

// =============================================================================
// Tunables
// =============================================================================

const (
	// Cost-per-second baseline. Costs are in "seconds-equivalent" so the
	// planner can compare walking, queuing, riding, and skiing uniformly.
	walkSpeedMps = 2.0
	skiSpeedMps  = 10.0 // average descent speed for cost estimation; the
	// L1 controller ultimately decides actual speed
	queueSlotSec = 8.0 // average wait per slot in line

	// MaxQueuePersons is the hard cap on queue depth a guest will tolerate
	// when planning. Queues longer than this cause JoinQueue's precondition
	// to fail, forcing the planner to seek an alternative lift. The cap is
	// bypassed when Patience < 0.05 so that GoHome routing can still ride
	// up and exit a lift base.
	MaxQueuePersons = 20

	// Lift-novelty bonus. First ride of a lift is "free"; each repeat ride
	// adds repeatPenaltyPerRide to RideLift's cost, capped so a much-ridden
	// lift never becomes prohibitive. Geometric decay would also work and
	// produces a softer falloff, but linear is easier to reason about and
	// the cap saturates quickly enough that the difference is academic.
	repeatPenaltyPerRide = 12.0
	repeatPenaltyCap     = 60.0

	// Rest duration constant — Rest restores Energy to ~full in one action.
	// Modeled as a chunky atomic action rather than a series of timed
	// recovery ticks so the planner doesn't need to chain dozens of small
	// rest actions to satisfy the Rest goal.
	restDurationSec = 60.0

	// Minimum vertical drop for a SkiTo* action to be applicable. Below
	// this, the destination is effectively at the same elevation as the
	// lift top, and skiing-to-it is degenerate. Keeps the action graph
	// from generating "ski to the lift you just unloaded at."
	minDescentMeters = 20.0
)

// =============================================================================
// Action types
// =============================================================================

// WalkToLift moves the agent from a ground anchor (parking or lodge) to a
// lift base. Walking — used when there's no useful descent line to the
// lift base from the agent's current position.
type WalkToLift struct{ LiftID uint64 }

func (a *WalkToLift) Name() string {
	return fmt.Sprintf("WalkToLift(%d)", a.LiftID)
}

func (a *WalkToLift) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed || s.OnLift != 0 || s.Queued != 0 || s.AtLiftBase != 0 || s.AtLiftTop != 0 {
		return false
	}
	if findLift(w, a.LiftID) == nil {
		return false
	}
	// Beginners and intermediates won't walk to a lift that has no trails
	// matching their ability. Advanced+ are willing to free-roam from any lift.
	if diff := skillDiff(s.Skill); diff != 0 {
		if !w.ServicesForLift(a.LiftID).Has(diff) {
			return false
		}
	}
	return true
}

func (a *WalkToLift) Apply(s *WorldSnapshot, w *world.World) {
	l := findLift(w, a.LiftID)
	if l == nil {
		return
	}
	s.Pos = mgl32.Vec3{l.Base[0], s.Pos[1], l.Base[1]}
	s.AtLodge = 0
	s.AtParking = 0
	s.AtLiftBase = l.ID
}

func (a *WalkToLift) Cost(s *WorldSnapshot, w *world.World) float32 {
	l := findLift(w, a.LiftID)
	if l == nil {
		return math.MaxFloat32
	}
	return distXZ(s.Pos, l.Base[0], l.Base[1]) / walkSpeedMps
}

// JoinQueue transitions the agent from AtLiftBase to Queued. Cost grows
// with queue length so the planner avoids long lines when alternatives
// exist
type JoinQueue struct{ LiftID uint64 }

func (a *JoinQueue) Name() string {
	return fmt.Sprintf("JoinQueue(%d)", a.LiftID)
}

func (a *JoinQueue) Precondition(s *WorldSnapshot, w *world.World) bool {
	l := findLift(w, a.LiftID)
	if !(!s.Removed && s.AtLiftBase == a.LiftID && l != nil && l.Open) {
		return false
	}
	if diff := skillDiff(s.Skill); diff != 0 {
		if !w.ServicesForLift(a.LiftID).Has(diff) {
			return false
		}
	}
	// Reject if the queue is too long, unless patience is already exhausted.
	// The exhausted exception keeps GoHome routing functional: a guest leaving
	// the mountain still needs to join a queue and ride up to exit a lift base.
	if s.Patience >= 0.05 && len(l.Queue) > MaxQueuePersons {
		return false
	}
	return true
}

func (a *JoinQueue) Apply(s *WorldSnapshot, w *world.World) {
	s.AtLiftBase = 0
	s.Queued = a.LiftID
}

func (a *JoinQueue) Cost(s *WorldSnapshot, w *world.World) float32 {
	l := findLift(w, a.LiftID)
	if l == nil {
		return math.MaxFloat32
	}
	return float32(len(l.Queue)) * queueSlotSec
}

// RideLift is folded board + ride + unload: the planner doesn't see the
// boarding step separately because BoardChair has no game-state effect
// beyond what RideLift's apply already captures. Effects: AtLiftTop set,
// Queued/OnLift cleared, RidenLifts incremented (which is the only
// novelty signal in the MVP).
type RideLift struct{ LiftID uint64 }

func (a *RideLift) Name() string {
	return fmt.Sprintf("RideLift(%d)", a.LiftID)
}

func (a *RideLift) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed {
		return false
	}
	return s.Queued == a.LiftID || s.OnLift == a.LiftID
}

func (a *RideLift) Apply(s *WorldSnapshot, w *world.World) {
	s.Queued = 0
	s.OnLift = 0
	s.AtLiftTop = a.LiftID
	s.RidenLifts = ai.AddRide(s.RidenLifts, a.LiftID)
}

func (a *RideLift) Cost(s *WorldSnapshot, w *world.World) float32 {
	l := findLift(w, a.LiftID)
	if l == nil {
		return math.MaxFloat32
	}
	ride := l.LoopLength() / (2 * l.Speed)
	// Repeat penalty: 0 for the first ride, ramps to repeatPenaltyCap.
	count := ai.RideCountOf(s.RidenLifts, a.LiftID)
	penalty := float32(count) * repeatPenaltyPerRide
	if penalty > repeatPenaltyCap {
		penalty = repeatPenaltyCap
	}
	return ride + penalty
}

// SkiToLift descends from a lift top to another lift's base. The base
// must be at least minDescentMeters below the source lift top — gravity-
// gated reachability stands in for explicit trails in the MVP.
type SkiToLift struct{ LiftID uint64 }

func (a *SkiToLift) Name() string {
	return fmt.Sprintf("SkiToLift(%d)", a.LiftID)
}

func (a *SkiToLift) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed || s.AtLiftTop == 0 {
		return false
	}
	src := findLift(w, s.AtLiftTop)
	dst := findLift(w, a.LiftID)
	if src == nil || dst == nil {
		return false
	}
	return liftTopElev(w, src)-liftBaseElev(w, dst) >= minDescentMeters
}

func (a *SkiToLift) Apply(s *WorldSnapshot, w *world.World) {
	l := findLift(w, a.LiftID)
	if l == nil {
		return
	}
	s.AtLiftTop = 0
	s.AtLiftBase = l.ID
	s.Pos = mgl32.Vec3{l.Base[0], s.Pos[1], l.Base[1]}
}

func (a *SkiToLift) Cost(s *WorldSnapshot, w *world.World) float32 {
	src := findLift(w, s.AtLiftTop)
	dst := findLift(w, a.LiftID)
	if src == nil || dst == nil {
		return math.MaxFloat32
	}
	return distXZ(mgl32.Vec3{src.Top[0], 0, src.Top[1]}, dst.Base[0], dst.Base[1]) / skiSpeedMps
}

// SkiToLodge descends from a lift top to a lodge. Used in plans that
// satisfy the Rest goal.
type SkiToLodge struct{ LodgeID uint64 }

func (a *SkiToLodge) Name() string {
	return fmt.Sprintf("SkiToLodge(%d)", a.LodgeID)
}

func (a *SkiToLodge) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed || s.AtLiftTop == 0 {
		return false
	}
	src := findLift(w, s.AtLiftTop)
	dst := findBuilding(w, a.LodgeID, world.BuildingLodge)
	if src == nil || dst == nil {
		return false
	}
	return liftTopElev(w, src)-buildingElev(w, dst) >= minDescentMeters
}

func (a *SkiToLodge) Apply(s *WorldSnapshot, w *world.World) {
	b := findBuilding(w, a.LodgeID, world.BuildingLodge)
	if b == nil {
		return
	}
	s.AtLiftTop = 0
	s.AtLodge = b.ID
	s.Pos = mgl32.Vec3{b.Pos[0], s.Pos[1], b.Pos[1]}
}

func (a *SkiToLodge) Cost(s *WorldSnapshot, w *world.World) float32 {
	src := findLift(w, s.AtLiftTop)
	dst := findBuilding(w, a.LodgeID, world.BuildingLodge)
	if src == nil || dst == nil {
		return math.MaxFloat32
	}
	return distXZ(mgl32.Vec3{src.Top[0], 0, src.Top[1]}, dst.Pos[0], dst.Pos[1]) / skiSpeedMps
}

// SkiToParking descends from a lift top to a parking lot. The terminal
// SkiTo* used by GoHome plans before Depart.
type SkiToParking struct{ LotID uint64 }

func (a *SkiToParking) Name() string {
	return fmt.Sprintf("SkiToParking(%d)", a.LotID)
}

func (a *SkiToParking) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed || s.AtLiftTop == 0 {
		return false
	}
	src := findLift(w, s.AtLiftTop)
	dst := findBuilding(w, a.LotID, world.BuildingParking)
	if src == nil || dst == nil {
		return false
	}
	return liftTopElev(w, src)-buildingElev(w, dst) >= minDescentMeters
}

func (a *SkiToParking) Apply(s *WorldSnapshot, w *world.World) {
	b := findBuilding(w, a.LotID, world.BuildingParking)
	if b == nil {
		return
	}
	s.AtLiftTop = 0
	s.AtParking = b.ID
	s.Pos = mgl32.Vec3{b.Pos[0], s.Pos[1], b.Pos[1]}
}

func (a *SkiToParking) Cost(s *WorldSnapshot, w *world.World) float32 {
	src := findLift(w, s.AtLiftTop)
	dst := findBuilding(w, a.LotID, world.BuildingParking)
	if src == nil || dst == nil {
		return math.MaxFloat32
	}
	return distXZ(mgl32.Vec3{src.Top[0], 0, src.Top[1]}, dst.Pos[0], dst.Pos[1]) / skiSpeedMps
}

// RestAtLodge is an atomic recovery action — one application restores
// Energy to near full. Models a single ~minute-long stop at a lodge
// rather than chaining many small recovery ticks, which would balloon
// plan length.
type RestAtLodge struct{ LodgeID uint64 }

func (a *RestAtLodge) Name() string {
	return fmt.Sprintf("RestAtLodge(%d)", a.LodgeID)
}

func (a *RestAtLodge) Precondition(s *WorldSnapshot, w *world.World) bool {
	return !s.Removed && s.AtLodge == a.LodgeID
}

func (a *RestAtLodge) Apply(s *WorldSnapshot, w *world.World) {
	s.Patience = 1
}

func (a *RestAtLodge) Cost(s *WorldSnapshot, w *world.World) float32 {
	return restDurationSec
}

// Depart is the terminal action that removes the agent from the sim.
// Precondition is AtParking; effect is Removed = true.
type Depart struct{ LotID uint64 }

func (a *Depart) Name() string {
	return fmt.Sprintf("Depart(%d)", a.LotID)
}

func (a *Depart) Precondition(s *WorldSnapshot, w *world.World) bool {
	return !s.Removed && s.AtParking == a.LotID
}

func (a *Depart) Apply(s *WorldSnapshot, w *world.World) {
	s.Removed = true
}

func (a *Depart) Cost(s *WorldSnapshot, w *world.World) float32 {
	return 0
}

// =============================================================================
// Action enumeration
// =============================================================================

// ApplicableActions enumerates every action whose precondition holds at
// snapshot s. The planner expands the current frontier by calling this
// and applying each result's Apply to a Cloned snapshot. Action count is
// O(lifts + lodges + parking + trail edges) — fine for small resort scales.
func ApplicableActions(s *WorldSnapshot, w *world.World) []Action {
	out := make([]Action, 0, 8)
	// Walk to any lift base from a ground anchor.
	if !s.Removed && s.OnLift == 0 && s.Queued == 0 && s.AtLiftBase == 0 && s.AtLiftTop == 0 {
		for _, l := range w.Lifts {
			a := &WalkToLift{LiftID: l.ID}
			if a.Precondition(s, w) {
				out = append(out, a)
			}
		}
	}
	// Queue / ride at the lift base, top, or while queued/on-lift.
	if s.AtLiftBase != 0 {
		out = append(out, &JoinQueue{LiftID: s.AtLiftBase})
	}
	if s.Queued != 0 {
		out = append(out, &RideLift{LiftID: s.Queued})
	}
	if s.OnLift != 0 {
		out = append(out, &RideLift{LiftID: s.OnLift})
	}

	// Trail-based descents from any anchor that has trail edges.
	out = trailActions(out, s, w)

	// Free-roam ski-down from a lift top (fallback; penalised when trail
	// alternatives exist from this anchor).
	if s.AtLiftTop != 0 {
		for _, l := range w.Lifts {
			a := &SkiToLift{LiftID: l.ID}
			if a.Precondition(s, w) {
				out = append(out, a)
			}
		}
		for _, b := range w.Buildings {
			switch b.Type {
			case world.BuildingLodge:
				a := &SkiToLodge{LodgeID: b.ID}
				if a.Precondition(s, w) {
					out = append(out, a)
				}
			case world.BuildingParking:
				a := &SkiToParking{LotID: b.ID}
				if a.Precondition(s, w) {
					out = append(out, a)
				}
			}
		}
	}

	// At a lodge or parking — rest or depart.
	if s.AtLodge != 0 {
		out = append(out, &RestAtLodge{LodgeID: s.AtLodge})
	}
	if s.AtParking != 0 {
		out = append(out, &Depart{LotID: s.AtParking})
	}
	return out
}

// =============================================================================
// Storage handoff — translate concrete Actions to plain-data ai.PlanAction
// =============================================================================

// ToPlanActions translates a planner-emitted Action slice into the leaf
// ai package's PlanAction records so the result can live on
// world.Guest.Plan without forcing world to import goap. Walks the plan
// applying each step to a snapshot copy so the per-step Cost reflects
// the state it was costed against during search.
func ToPlanActions(actions []Action, snap WorldSnapshot, w *world.World) []ai.PlanAction {
	if len(actions) == 0 {
		return nil
	}
	out := make([]ai.PlanAction, 0, len(actions))
	step := snap.Clone()
	for _, a := range actions {
		pa := ai.PlanAction{Cost: a.Cost(&step, w)}
		switch t := a.(type) {
		case *WalkToLift:
			pa.Kind = ai.ActWalkToLift
			pa.LiftID = t.LiftID
		case *JoinQueue:
			pa.Kind = ai.ActJoinQueue
			pa.LiftID = t.LiftID
		case *RideLift:
			pa.Kind = ai.ActRideLift
			pa.LiftID = t.LiftID
		case *SkiToLift:
			pa.Kind = ai.ActSkiToLift
			pa.LiftID = t.LiftID
		case *SkiToLodge:
			pa.Kind = ai.ActSkiToLodge
			pa.BldgID = t.LodgeID
		case *SkiToParking:
			pa.Kind = ai.ActSkiToParking
			pa.BldgID = t.LotID
		case *RestAtLodge:
			pa.Kind = ai.ActRestAtLodge
			pa.BldgID = t.LodgeID
		case *Depart:
			pa.Kind = ai.ActDepart
			pa.BldgID = t.LotID
		case *SkiTrail:
			pa.Kind = ai.ActSkiTrail
			pa.TrailID = t.TrailID // via trail (display); overridden for trail-to-trail
			switch t.ToKind {
			case world.KindLiftBase:
				pa.LiftID = t.ToID
			case world.KindBuilding:
				pa.BldgID = t.ToID
			case world.KindTrail:
				pa.TrailID = t.ToID // destination trail — used by planActionComplete
			}
		}
		out = append(out, pa)
		a.Apply(&step, w)
	}
	return out
}

// PlanActionLabel renders an ai.PlanAction with the same name + entity
// label formatting goap.DisplayName uses for concrete Actions. Used by
// the HUD which now reads plan steps from the agent rather than from
// fresh planner output.
func PlanActionLabel(pa ai.PlanAction, w *world.World) string {
	switch pa.Kind {
	case ai.ActWalkToLift:
		return "WalkToLift(" + liftLabel(w, pa.LiftID) + ")"
	case ai.ActJoinQueue:
		return "JoinQueue(" + liftLabel(w, pa.LiftID) + ")"
	case ai.ActRideLift:
		return "RideLift(" + liftLabel(w, pa.LiftID) + ")"
	case ai.ActSkiToLift:
		return "SkiToLift(" + liftLabel(w, pa.LiftID) + ")"
	case ai.ActSkiToLodge:
		return "SkiToLodge(" + buildingLabel(w, pa.BldgID) + ")"
	case ai.ActSkiToParking:
		return "SkiToParking(" + buildingLabel(w, pa.BldgID) + ")"
	case ai.ActRestAtLodge:
		return "RestAtLodge(" + buildingLabel(w, pa.BldgID) + ")"
	case ai.ActDepart:
		return "Depart(" + buildingLabel(w, pa.BldgID) + ")"
	case ai.ActSkiTrail:
		return skiTrailPlanActionLabel(pa, w)
	}
	return "—"
}

// =============================================================================
// Display
// =============================================================================

// DisplayName returns a human-readable form of an action's Name() with
// entity IDs swapped for labels: lift names where set (PlaceLift auto-
// assigns Lift1, Lift2, ...), building positions otherwise. The HUD
// uses this; Name() stays raw so logs and recorder traces remain stable
// even when lifts get renamed mid-session.
func DisplayName(a Action, w *world.World) string {
	switch act := a.(type) {
	case *WalkToLift:
		return "WalkToLift(" + liftLabel(w, act.LiftID) + ")"
	case *JoinQueue:
		return "JoinQueue(" + liftLabel(w, act.LiftID) + ")"
	case *RideLift:
		return "RideLift(" + liftLabel(w, act.LiftID) + ")"
	case *SkiToLift:
		return "SkiToLift(" + liftLabel(w, act.LiftID) + ")"
	case *SkiToLodge:
		return "SkiToLodge(" + buildingLabel(w, act.LodgeID) + ")"
	case *SkiToParking:
		return "SkiToParking(" + buildingLabel(w, act.LotID) + ")"
	case *RestAtLodge:
		return "RestAtLodge(" + buildingLabel(w, act.LodgeID) + ")"
	case *Depart:
		return "Depart(" + buildingLabel(w, act.LotID) + ")"
	case *SkiTrail:
		return skiTrailDisplayName(act, w)
	}
	return a.Name()
}

func liftLabel(w *world.World, id uint64) string {
	if l := findLift(w, id); l != nil && l.Name != "" {
		return l.Name
	}
	return fmt.Sprintf("#%d", id)
}

func buildingLabel(w *world.World, id uint64) string {
	for _, b := range w.Buildings {
		if b.ID != id {
			continue
		}
		// Lodges and parking lots don't have names yet; identify by type
		// + ID so the HUD reads "Lodge #4" / "Parking #2".
		switch b.Type {
		case world.BuildingLodge:
			return fmt.Sprintf("Lodge#%d", id)
		case world.BuildingParking:
			return fmt.Sprintf("Lot#%d", id)
		}
	}
	return fmt.Sprintf("#%d", id)
}

// =============================================================================
// Helpers
// =============================================================================

func findLift(w *world.World, id uint64) *world.Lift {
	for _, l := range w.Lifts {
		if l.ID == id {
			return l
		}
	}
	return nil
}

func findBuilding(w *world.World, id uint64, typ world.BuildingType) *world.Building {
	for _, b := range w.Buildings {
		if b.ID == id && b.Type == typ {
			return b
		}
	}
	return nil
}

func liftTopElev(w *world.World, l *world.Lift) float32 {
	return w.Terrain.InterpolatedSurfaceElevationAt(l.Top[0], l.Top[1])
}

func liftBaseElev(w *world.World, l *world.Lift) float32 {
	return w.Terrain.InterpolatedSurfaceElevationAt(l.Base[0], l.Base[1])
}

func buildingElev(w *world.World, b *world.Building) float32 {
	return w.Terrain.InterpolatedSurfaceElevationAt(b.Pos[0], b.Pos[1])
}

func distXZ(p mgl32.Vec3, x, z float32) float32 {
	dx := p[0] - x
	dz := p[2] - z
	return float32(math.Sqrt(float64(dx*dx + dz*dz)))
}
