package goap

import (
	"strings"
	"testing"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

// buildSmokeWorld constructs a tiny resort: 1 parking lot at (50, 50),
// 1 lodge at (60, 60), and 2 lifts whose tops sit ~80 m above their bases
// so SkiTo* preconditions are satisfied. Terrain is flat (NewTerrain
// defaults to elevation 0) so we hand-tweak cells under each lift top
// to give them real elevation drop.
func buildSmokeWorld(t *testing.T) (*world.World, *world.Building, *world.Lift, *world.Lift) {
	t.Helper()
	terrain := world.NewTerrain(60, 60)
	w := world.NewWorld(terrain)

	// Two lifts: A at base (50, 50) → top (150, 50); B at base (150, 50) →
	// top (50, 200). Top cells get +80 m elevation so SkiToLift /
	// SkiToParking pass the 20 m descent gate.
	raiseCell(terrain, 30, 10, 80) // lift A top cell ~ (150,50)/5 = (30,10)
	raiseCell(terrain, 10, 40, 80) // lift B top cell ~ (50,200)/5 = (10,40)

	parking := w.PlaceBuildingType(world.BuildingParking, 50, 50)
	_ = w.PlaceBuildingType(world.BuildingLodge, 60, 60)
	liftA := w.PlaceLift(world.LiftDouble, 50, 50, 150, 50)
	liftB := w.PlaceLift(world.LiftDouble, 150, 50, 50, 200)

	return w, parking, liftA, liftB
}

func raiseCell(t *world.Terrain, x, z int, dh float32) {
	if !t.InBounds(x, z) {
		return
	}
	t.Cells[x][z].GroundElevation = dh
	t.Cells[x][z].NaturalElev = dh
}

// TestPlanFromParking is the end-to-end smoke: agent at parking with
// fresh Energy. Selected goal should be KeepSkiing or Explore (both
// satisfiable by riding a lift), and the plan should chain WalkToLift
// → JoinQueue → RideLift.
func TestPlanFromParking(t *testing.T) {
	w, parking, _, _ := buildSmokeWorld(t)

	snap := WorldSnapshot{
		Pos:       mgl32.Vec3{parking.Pos[0], 0, parking.Pos[1]},
		Energy:    1.0,
		AtParking: parking.ID,
	}
	goal := SelectGoal(&snap, w)
	if goal == nil {
		t.Fatal("SelectGoal returned nil at full energy with unridden lifts")
	}
	// At full energy + no rides yet, weight order: Explore (1.0) >
	// KeepSkiing (1.0) > GoHome (0.4 satiation only) > Rest (0).
	// The tie between Explore and KeepSkiing is broken by iteration
	// order in AllGoals — KeepSkiing comes first so it wins on equal
	// weight. Both produce a valid "ride a lift" plan, so accept either.
	if goal.Name() != "KeepSkiing" && goal.Name() != "Explore" {
		t.Errorf("expected KeepSkiing or Explore, got %q", goal.Name())
	}

	p := NewPlanner()
	plan := p.Plan(snap, goal, w)
	if plan == nil {
		t.Fatalf("planner returned no plan for %s from parking", goal.Name())
	}
	if len(plan) < 3 {
		t.Fatalf("expected ≥3 actions to ride a lift, got %d: %v", len(plan), planNames(plan))
	}
	// Head sequence should be WalkToLift → JoinQueue → RideLift.
	wantPrefixes := []string{"WalkToLift", "JoinQueue", "RideLift"}
	for i, want := range wantPrefixes {
		if !strings.HasPrefix(plan[i].Name(), want) {
			t.Errorf("action %d: want prefix %q, got %q", i, want, plan[i].Name())
		}
	}
}

// TestExplorePrefersUnridden: once one lift is in RidenLifts, Explore
// should produce a plan whose RideLift targets the *other* lift.
func TestExplorePrefersUnridden(t *testing.T) {
	w, _, liftA, liftB := buildSmokeWorld(t)

	// Agent at top of A (just unloaded), already rode A once.
	snap := WorldSnapshot{
		Pos:        mgl32.Vec3{liftA.Top[0], 0, liftA.Top[1]},
		Energy:     0.7,
		AtLiftTop:  liftA.ID,
		RidenLifts: map[uint64]int{liftA.ID: 1},
	}

	p := NewPlanner()
	plan := p.Plan(snap, Explore{}, w)
	if plan == nil {
		t.Fatal("Explore returned no plan with one unridden lift remaining")
	}
	// Find the RideLift in the plan and check its target.
	var rideID uint64
	for _, a := range plan {
		if r, ok := a.(*RideLift); ok {
			rideID = r.LiftID
			break
		}
	}
	if rideID != liftB.ID {
		t.Errorf("Explore should ride the unridden lift B (%d); got plan: %v", liftB.ID, planNames(plan))
	}
}

// TestRestAtLowEnergy: low Energy makes Rest dominate, and the planner
// should produce a plan that ends in RestAtLodge.
func TestRestAtLowEnergy(t *testing.T) {
	w, _, liftA, _ := buildSmokeWorld(t)

	snap := WorldSnapshot{
		Pos:       mgl32.Vec3{liftA.Top[0], 0, liftA.Top[1]},
		Energy:    0.1,
		AtLiftTop: liftA.ID,
	}
	goal := SelectGoal(&snap, w)
	if goal.Name() != "Rest" {
		t.Fatalf("expected Rest at Energy=0.1, got %s", goal.Name())
	}
	p := NewPlanner()
	plan := p.Plan(snap, goal, w)
	if plan == nil {
		t.Fatal("Rest plan came back nil")
	}
	last := plan[len(plan)-1]
	if !strings.HasPrefix(last.Name(), "RestAtLodge") {
		t.Errorf("Rest plan should end in RestAtLodge; got tail %q (full plan: %v)", last.Name(), planNames(plan))
	}
}

func planNames(p []Action) []string {
	out := make([]string, len(p))
	for i, a := range p {
		out[i] = a.Name()
	}
	return out
}
