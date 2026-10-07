package goap

import (
	"testing"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

func TestNeedPreempts(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(8, 8))
	snap := WorldSnapshot{Patience: 1, Energy: 1}
	plan := ai.Plan{GoalName: "KeepSkiing", Pressing: PressingNeeds(&snap, w)}
	if NeedPreempts(&snap, w, &plan) {
		t.Fatal("no need is pressing yet")
	}

	snap.Need[ai.NeedHunger] = 0.8
	if !NeedPreempts(&snap, w, &plan) {
		t.Fatal("hunger crossing its threshold should preempt KeepSkiing")
	}

	// A plan made while already hungry (e.g. no food court reachable)
	// isn't preempted again for the same need.
	stale := ai.Plan{GoalName: "KeepSkiing", Pressing: PressingNeeds(&snap, w)}
	if NeedPreempts(&snap, w, &stale) {
		t.Fatal("hunger was already pressing at plan time")
	}

	// Thirst that hasn't crossed its threshold (a quarter left) doesn't
	// preempt; once it has, it outweighs a keen skier's KeepSkiing, as
	// hunger does.
	snap.Need[ai.NeedHunger], snap.Need[ai.NeedThirst] = 0, 0.7
	if NeedPreempts(&snap, w, &plan) {
		t.Fatal("thirst under its threshold shouldn't preempt KeepSkiing")
	}
	snap.Need[ai.NeedThirst] = 0.8
	if !NeedPreempts(&snap, w, &plan) {
		t.Fatal("pressing thirst should preempt KeepSkiing")
	}
	snap.Need[ai.NeedThirst] = 0.96
	if !NeedPreempts(&snap, w, &plan) {
		t.Fatal("critical thirst should preempt KeepSkiing")
	}
}
