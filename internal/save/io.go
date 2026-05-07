package save

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

// SaveInfo describes one entry in the user's saves directory.
type SaveInfo struct {
	Name    string    // file basename without extension
	Path    string    // absolute path on disk
	ModTime time.Time // last-modified time, used for newest-first sorting
}

// SavesDir returns the directory holding the user's named saves. The
// directory is created on access so callers can write to it directly.
// Falls back to ./mountain-mogul-saves when the home directory is unknown.
func SavesDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		dir := "mountain-mogul-saves"
		_ = os.MkdirAll(dir, 0o755)
		return dir
	}
	dir := filepath.Join(home, ".mountain-mogul", "saves")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// ListSaves returns every save file in SavesDir, sorted newest-first by
// modification time. Non-JSON entries are ignored.
func ListSaves() []SaveInfo {
	dir := SavesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]SaveInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, SaveInfo{
			Name:    strings.TrimSuffix(e.Name(), ".json"),
			Path:    filepath.Join(dir, e.Name()),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out
}

// MostRecentSave returns the path of the newest save and ok=true, or
// ok=false when no saves exist. Drives the start menu's Continue button.
func MostRecentSave() (string, bool) {
	saves := ListSaves()
	if len(saves) == 0 {
		return "", false
	}
	return saves[0].Path, true
}

// SaveAs writes the world to a file inside SavesDir named after `name`. Any
// path separators in `name` are stripped so the write can't escape the dir.
// Returns the final path written.
func SaveAs(name string, w *world.World) (string, error) {
	clean := SanitizeSaveName(name)
	if clean == "" {
		clean = "save"
	}
	path := filepath.Join(SavesDir(), clean+".json")
	if err := SaveScenario(path, w); err != nil {
		return "", err
	}
	return path, nil
}

// SanitizeSaveName strips path separators, leading dots, and trims spaces so
// `name` resolves to a single file inside SavesDir. Empty result is allowed
// (the caller decides whether that's an error).
func SanitizeSaveName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	name = strings.TrimLeft(name, ".")
	return name
}

// DefaultSaveName returns a timestamp-based default like "save-2026-05-06-1530"
// suitable for pre-filling the save-name prompt.
func DefaultSaveName() string {
	return "save-" + time.Now().Format("2006-01-02-1504")
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
			ID:            b.ID,
			X:             b.Pos[0],
			Z:             b.Pos[1],
			Rotation:      b.Rotation,
			MeanSpawnRate: b.MeanSpawnRate,
			SkierCount:    b.SkierCount,
		}
	}

	lifts := make([]LiftData, len(w.Lifts))
	for i, l := range w.Lifts {
		chairs := make([]ChairData, len(l.Chairs))
		for ci, c := range l.Chairs {
			cd := ChairData{Progress: c.Progress}
			for pi, pax := range c.Passengers {
				if pax != nil {
					cd.PassengerIDs[pi] = pax.ID
				}
			}
			chairs[ci] = cd
		}
		queueIDs := make([]uint64, 0, len(l.Queue))
		for _, a := range l.Queue {
			if a != nil {
				queueIDs = append(queueIDs, a.ID)
			}
		}
		lifts[i] = LiftData{
			ID:       l.ID,
			BaseX:    l.Base[0],
			BaseZ:    l.Base[1],
			TopX:     l.Top[0],
			TopZ:     l.Top[1],
			Speed:    l.Speed,
			Chairs:   chairs,
			QueueIDs: queueIDs,
		}
	}

	agents := make([]AgentData, len(w.Agents))
	for i, a := range w.Agents {
		agents[i] = AgentData{
			ID:       a.ID,
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

	// Restore objects (no cross-references, fresh IDs are fine).
	for _, od := range data.Objects {
		obj := w.PlaceObject(world.ObjectType(od.Type), od.X, od.Z)
		obj.Rotation = od.Rotation
	}

	// Restore buildings, preserving IDs so agent.TargetID references stay
	// valid. Old saves without an `id` field fall back to a fresh ID.
	for _, bd := range data.Buildings {
		b := w.PlaceBuilding(bd.X, bd.Z)
		if bd.ID != 0 {
			b.ID = bd.ID
		}
		b.Rotation = bd.Rotation
		if bd.MeanSpawnRate > 0 {
			b.MeanSpawnRate = bd.MeanSpawnRate
		}
		if bd.SkierCount > 0 {
			b.SkierCount = bd.SkierCount
		}
	}

	// Restore lifts. Chair count is computed from cable length so it's
	// stable across save/load; we still copy progress + passenger refs
	// from the saved chair list. Queue IDs are resolved after agents
	// load below.
	for _, ld := range data.Lifts {
		lift := w.PlaceLift(ld.BaseX, ld.BaseZ, ld.TopX, ld.TopZ)
		if ld.ID != 0 {
			lift.ID = ld.ID
		}
		if ld.Speed >= 0.1 {
			lift.Speed = ld.Speed
		}
		// Restore chair Progress where the saved length matches; if the
		// chair count differs (e.g. a code change), keep the freshly
		// initialised even-spacing for the unmatched chairs.
		for ci := range lift.Chairs {
			if ci < len(ld.Chairs) {
				lift.Chairs[ci].Progress = ld.Chairs[ci].Progress
			}
		}
	}

	// Restore agents, preserving IDs so chair / queue references resolve.
	for _, ad := range data.Agents {
		var id uint64
		if ad.ID != 0 {
			id = ad.ID
		} else {
			id = w.NextID()
		}
		agent := &world.Agent{
			ID:       id,
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

	// Build an ID lookup so we can resolve chair-passenger and queue
	// references back to live *Agent pointers.
	agentByID := make(map[uint64]*world.Agent, len(w.Agents))
	for _, a := range w.Agents {
		agentByID[a.ID] = a
	}

	for li, ld := range data.Lifts {
		lift := w.Lifts[li]
		// Chairs: re-link passengers by ID. Drop refs that don't resolve.
		for ci := range lift.Chairs {
			if ci >= len(ld.Chairs) {
				break
			}
			for pi, pid := range ld.Chairs[ci].PassengerIDs {
				if pid == 0 {
					continue
				}
				if a := agentByID[pid]; a != nil {
					lift.Chairs[ci].Passengers[pi] = a
				}
			}
		}
		// Queue: rebuild in saved order, skipping unresolved IDs.
		for _, qid := range ld.QueueIDs {
			if a := agentByID[qid]; a != nil {
				lift.Queue = append(lift.Queue, a)
			}
		}
	}

	// Bump the world's ID counter past the highest restored ID so future
	// spawns don't collide.
	var maxID uint64
	for _, b := range w.Buildings {
		if b.ID > maxID {
			maxID = b.ID
		}
	}
	for _, l := range w.Lifts {
		if l.ID > maxID {
			maxID = l.ID
		}
	}
	for _, a := range w.Agents {
		if a.ID > maxID {
			maxID = a.ID
		}
	}
	w.SetMinNextID(maxID)

	return w
}
