package scene

import (
	"testing"

	"mountain-mogul/internal/world"
)

func TestSortScenarios(t *testing.T) {
	s := []scenarioEntry{
		{file: "zz-unordered"},
		{file: "kirkwood", info: world.ScenarioInfo{Name: "Kirkwood", Order: 3}},
		{file: "aa-unordered"},
		{file: "mrg", info: world.ScenarioInfo{Name: "Mad River Glen", Order: 2}},
		{file: "boreal", info: world.ScenarioInfo{Name: "Boreal", Order: 1, Tutorial: true}},
	}
	sortScenarios(s)
	want := []string{"Boreal", "Mad River Glen", "Kirkwood", "aa-unordered", "zz-unordered"}
	for i, e := range s {
		if e.label() != want[i] {
			t.Fatalf("position %d = %q, want %q (order %v)", i, e.label(), want[i], want)
		}
	}
}

func TestScenarioDetailsPromptResult(t *testing.T) {
	in := world.ScenarioInfo{Name: "Old", Difficulty: 5, Order: 2}
	var got *world.ScenarioInfo
	p := newScenarioDetailsPrompt(in, nil, nil, func(info world.ScenarioInfo, _ []world.Goal, _ []string) { got = &info }, func() {})
	p.fields[0].Text = "  Kirkwood "
	p.fields[1].Text = "Kirkwood, California"
	p.fields[2].Text = "Line one.\nLine two.\n"
	p.diffUp.Click() // clamps at the top of the scale
	p.orderDown.Click()
	p.tutorialBtn.Click()
	p.okBtn.Click()
	want := world.ScenarioInfo{
		Name: "Kirkwood", Location: "Kirkwood, California", Description: "Line one.\nLine two.",
		Difficulty: 5, Order: 1, Tutorial: true,
	}
	if got == nil || *got != want {
		t.Fatalf("OK returned %+v, want %+v", got, want)
	}
}
