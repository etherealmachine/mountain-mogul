package world

// MaxScenarioDifficulty is the top of the ScenarioInfo.Difficulty scale.
const MaxScenarioDifficulty = 5

// ScenarioInfo describes the scenario a world was authored as, or the one a
// game was started from. Set in the Scenario Editor, saved with the world,
// and carried into player saves so they remember which mountain they're on.
type ScenarioInfo struct {
	Name        string // display name; "" falls back to the file name
	Description string // the place, the challenge, and what's new; may hold newlines
	Location    string // resort and region, e.g. "Donner Pass, California"
	Difficulty  int    // 1 to MaxScenarioDifficulty; 0 = not set
	Order       int    // campaign position, ascending; 0 = unordered (sorts last)
	Tutorial    bool   // shown first in the picker and marked as the tutorial
}

// GeoBounds is the latitude/longitude box, in degrees, an imported
// terrain covers: cell column 0 is MinLon and Width-1 is MaxLon; row 0
// is MaxLat and Height-1 is MinLat.
type GeoBounds struct {
	MinLat, MaxLat, MinLon, MaxLon float64
}
