package save

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

// SaveSlotPath returns the path to the canonical user save slot. Falls back
// to the working directory if the user's home directory cannot be determined.
// The parent directory is created on access so callers can write directly.
func SaveSlotPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "mountain-mogul-save.json"
	}
	dir := filepath.Join(home, ".mountain-mogul")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "save.json")
}

// SaveSlotExists reports whether a save file exists at SaveSlotPath.
func SaveSlotExists() bool {
	_, err := os.Stat(SaveSlotPath())
	return err == nil
}

// SaveScenario marshals the world to JSON and writes it to path.
func SaveScenario(path string, w *world.World) error {
	data := worldToData(w)
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0644)
}

// LoadScenario reads and parses a scenario JSON file, returning a World.
func LoadScenario(path string) (*world.World, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data ScenarioData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return dataToWorld(data), nil
}

func worldToData(w *world.World) ScenarioData {
	t := w.Terrain
	cells := make([]CellData, 0, t.Width*t.Height)
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			c := t.Cells[x][z]
			cells = append(cells, CellData{
				Elevation:   c.Elevation,
				Groomed:     c.Groomed,
				SnowDepth:   c.SnowDepth,
				TreeDensity: c.TreeDensity,
			})
		}
	}

	objects := make([]ObjectData, len(w.Objects))
	for i, obj := range w.Objects {
		objects[i] = ObjectData{
			Type:     uint8(obj.Type),
			X:        obj.Pos[0],
			Z:        obj.Pos[1],
			Rotation: obj.Rotation,
		}
	}

	buildings := make([]BuildingData, len(w.Buildings))
	for i, b := range w.Buildings {
		buildings[i] = BuildingData{
			X:             b.Pos[0],
			Z:             b.Pos[1],
			Rotation:      b.Rotation,
			MeanSpawnRate: b.MeanSpawnRate,
			SkierCount:    b.SkierCount,
		}
	}

	lifts := make([]LiftData, len(w.Lifts))
	for i, l := range w.Lifts {
		lifts[i] = LiftData{
			BaseX: l.Base[0],
			BaseZ: l.Base[1],
			TopX:  l.Top[0],
			TopZ:  l.Top[1],
			Speed: l.Speed,
		}
	}

	agents := make([]AgentData, len(w.Agents))
	for i, a := range w.Agents {
		agents[i] = AgentData{
			Pos:      [3]float32{a.Pos[0], a.Pos[1], a.Pos[2]},
			Heading:  a.Heading,
			Path:     a.Path,
			PathIdx:  a.PathIdx,
			Speed:    a.Speed,
			TargetID: a.TargetID,
			OnLiftID: a.OnLiftID,
			Queued:   a.Queued,
		}
	}

	return ScenarioData{
		Name:      "scenario",
		Width:     t.Width,
		Height:    t.Height,
		Cells:     cells,
		Objects:   objects,
		Buildings: buildings,
		Lifts:     lifts,
		Agents:    agents,
	}
}

func dataToWorld(data ScenarioData) *world.World {
	t := world.NewTerrain(data.Width, data.Height)

	// Restore cells
	idx := 0
	for x := 0; x < data.Width; x++ {
		for z := 0; z < data.Height; z++ {
			if idx < len(data.Cells) {
				c := data.Cells[idx]
				t.Cells[x][z].Elevation = c.Elevation
				t.Cells[x][z].Groomed = c.Groomed
				t.Cells[x][z].SnowDepth = c.SnowDepth
				if t.Cells[x][z].SnowDepth == 0 {
					t.Cells[x][z].SnowDepth = 1.0 // default
				}
				t.Cells[x][z].TreeDensity = c.TreeDensity
			}
			idx++
		}
	}

	w := world.NewWorld(t)

	// Restore objects
	for _, od := range data.Objects {
		obj := w.PlaceObject(world.ObjectType(od.Type), od.X, od.Z)
		obj.Rotation = od.Rotation
	}

	// Restore buildings
	for _, bd := range data.Buildings {
		b := w.PlaceBuilding(bd.X, bd.Z)
		b.Rotation = bd.Rotation
		if bd.MeanSpawnRate > 0 {
			b.MeanSpawnRate = bd.MeanSpawnRate
		}
		if bd.SkierCount > 0 {
			b.SkierCount = bd.SkierCount
		}
	}

	// Restore lifts
	for _, ld := range data.Lifts {
		lift := w.PlaceLift(ld.BaseX, ld.BaseZ, ld.TopX, ld.TopZ)
		// Guard against old saves that stored fractional speed (< 0.1) — treat as default.
		if ld.Speed >= 0.1 {
			lift.Speed = ld.Speed
		}
	}

	// Restore agents
	for _, ad := range data.Agents {
		agent := &world.Agent{
			ID:       w.NextID(),
			Pos:      mgl32.Vec3{ad.Pos[0], ad.Pos[1], ad.Pos[2]},
			Heading:  ad.Heading,
			Path:     ad.Path,
			PathIdx:  ad.PathIdx,
			Speed:    ad.Speed,
			TargetID: ad.TargetID,
			OnLiftID: ad.OnLiftID,
			Queued:   ad.Queued,
		}
		w.Agents = append(w.Agents, agent)
	}

	return w
}
