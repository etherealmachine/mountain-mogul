package goap

import (
	"testing"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

func TestNeedPreempts(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(8, 8))
	snap := WorldSnapshot{Patience: 1, Energy: 1, Hunger: 1, Thirst: 1}
	plan := ai.Plan{GoalName: "KeepSkiing", Pressing: PressingNeeds(&snap, w)}
	if NeedPreempts(&snap, w, &plan) {
		t.Fatal("no need is pressing yet")
	}

	snap.Hunger = 0.2
	if !NeedPreempts(&snap, w, &plan) {
		t.Fatal("hunger crossing its threshold should preempt KeepSkiing")
	}

	// A plan made while already hungry (e.g. no food court reachable)
	// isn't preempted again for the same need.
	stale := ai.Plan{GoalName: "KeepSkiing", Pressing: PressingNeeds(&snap, w)}
	if NeedPreempts(&snap, w, &stale) {
		t.Fatal("hunger was already pressing at plan time")
	}

	// Mild thirst weighs less than a keen skier's KeepSkiing.
	snap.Hunger, snap.Thirst = 1, 0.2
	if NeedPreempts(&snap, w, &plan) {
		t.Fatal("mild thirst shouldn't outweigh KeepSkiing")
	}
	snap.Thirst = 0.04
	if !NeedPreempts(&snap, w, &plan) {
		t.Fatal("critical thirst should preempt KeepSkiing")
	}
}
