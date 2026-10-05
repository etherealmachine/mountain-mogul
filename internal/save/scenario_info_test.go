package save

import (
	"path/filepath"
	"testing"

	"mountain-mogul/internal/world"
)

func TestScenarioInfoRoundTrip(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(16, 16))
	w.Scenario = world.ScenarioInfo{
		Name:        "Boreal",
		Description: "A small hill.\nRight off the interstate.",
		Location:    "Donner Pass, California",
		Difficulty:  1,
		Order:       1,
		Tutorial:    true,
	}
	path := filepath.Join(t.TempDir(), "s"+SaveExt)
	if err := SaveScenario(path, w, nil); err != nil {
		t.Fatal(err)
	}
	got, _, err := LoadScenario(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Scenario != w.Scenario {
		t.Errorf("loaded %+v, want %+v", got.Scenario, w.Scenario)
	}
	info, err := ReadScenarioInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	if info != w.Scenario {
		t.Errorf("ReadScenarioInfo = %+v, want %+v", info, w.Scenario)
	}
}

// Saves from before scenario names wrote the placeholder "scenario".
func TestLegacyScenarioNameIsBlank(t *testing.T) {
	data := worldToData(world.NewWorld(world.NewTerrain(8, 8)), false)
	data.Name = legacyScenarioName
	path := filepath.Join(t.TempDir(), "old"+SaveExt)
	if err := WriteScenarioData(path, data); err != nil {
		t.Fatal(err)
	}
	info, err := ReadScenarioInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "" {
		t.Errorf("legacy name read as %q, want blank", info.Name)
	}
	if got := dataToWorld(data).Scenario.Name; got != "" {
		t.Errorf("legacy name loaded as %q, want blank", got)
	}
}

func BenchmarkReadScenarioInfoTutorial(b *testing.B) {
	path := filepath.Join("..", "..", "assets", "scenarios", "boreal"+SaveExt)
	for i := 0; i < b.N; i++ {
		if _, err := ReadScenarioInfo(path); err != nil {
			b.Fatal(err)
		}
	}
}
