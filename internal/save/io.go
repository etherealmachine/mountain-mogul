package save

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/vmihailenco/msgpack/v5"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// SaveExt is the on-disk extension for the save format: msgpack-
// encoded ScenarioData wrapped in gzip. Cheap, compact, binary.
const SaveExt = ".save"

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
// modification time. Only `.save` files are considered — the binary
// format is the project's single save format.
func ListSaves() []SaveInfo {
	dir := SavesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]SaveInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), SaveExt) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, SaveInfo{
			Name:    strings.TrimSuffix(e.Name(), SaveExt),
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
// Returns the final path written. cam is optional — pass nil to skip
// camera persistence.
func SaveAs(name string, w *world.World, cam *CameraData) (string, error) {
	clean := SanitizeSaveName(name)
	if clean == "" {
		clean = "save"
	}
	path := filepath.Join(SavesDir(), clean+SaveExt)
	if err := SaveScenario(path, w, cam); err != nil {
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

// SaveScenario marshals the world and writes it to path. cam is
// optional; if non-nil, the orthographic camera state is captured so
// the scene reloads framed exactly as the player left it.
//
// Format: msgpack-encoded ScenarioData wrapped in a gzip stream. The
// msgpack encoder is configured to honour the existing `json:` struct
// tags so the on-wire schema stays the single source of truth. Gzip
// then crushes the long runs of identical-or-similar cell values
// (most cells in a scenario have default snow state) down to a tiny
// fraction of the original.
func SaveScenario(path string, w *world.World, cam *CameraData) error {
	data := worldToData(w)
	if cam != nil {
		c := *cam
		data.Camera = &c
	}
	return WriteScenarioData(path, data)
}

// WriteScenarioData writes an already-built ScenarioData to disk in
// the project's binary format. Used by SaveScenario and by the
// converter tool that produces bundled scenarios from external
// sources.
func WriteScenarioData(path string, data ScenarioData) error {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	enc := msgpack.NewEncoder(gz)
	enc.SetCustomStructTag("json")
	enc.UseCompactInts(true)
	enc.UseCompactFloats(true)
	if err := enc.Encode(data); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

// LoadScenario reads and parses a `.save` file, returning a World
// and, if the save included one, the camera snapshot. cam is nil for
// saves that predate camera persistence (or never had a camera set).
func LoadScenario(path string) (*world.World, *CameraData, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, fmt.Errorf("save %q: %w", path, err)
	}
	defer gz.Close()
	var data ScenarioData
	dec := msgpack.NewDecoder(gz)
	dec.SetCustomStructTag("json")
	if err := dec.Decode(&data); err != nil && err != io.EOF {
		return nil, nil, fmt.Errorf("save %q: %w", path, err)
	}
	return dataToWorld(data), data.Camera, nil
}

func worldToData(w *world.World) ScenarioData {
	t := w.Terrain
	cells := make([]CellData, 0, t.Width*t.Height)
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			c := t.Cells[x][z]
			cells = append(cells, CellData{
				Ground:      c.GroundElevation,
				Snow:        c.SnowAccumulation,
				Grooming:    c.Grooming,
				Packed:      c.Packed,
				Ice:         c.Ice,
				MogulSize:   c.MogulSize,
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
			ID:              b.ID,
			Type:            uint8(b.Type),
			X:               b.Pos[0],
			Z:               b.Pos[1],
			Rotation:        b.Rotation,
			Cats:            b.Cats,
			MaxCars:         b.MaxCars,
			CurrentCars:     b.CurrentCars,
			DrivewayNodeIDs: b.DrivewayNodeIDs,
		}
	}

	snowcats := make([]SnowcatData, len(w.Snowcats))
	for i, c := range w.Snowcats {
		snowcats[i] = SnowcatData{
			ID:         c.ID,
			ShedID:     c.ShedID,
			Pos:        [3]float32{c.Pos[0], c.Pos[1], c.Pos[2]},
			Heading:    c.Heading,
			TargetCell: c.TargetCell,
		}
	}

	lifts := make([]LiftData, len(w.Lifts))
	for i, l := range w.Lifts {
		chairs := make([]ChairData, len(l.Chairs))
		for ci, c := range l.Chairs {
			cd := ChairData{
				Progress:     c.Progress,
				PassengerIDs: make([]uint64, len(c.Passengers)),
			}
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
			ID:          l.ID,
			Type:        uint8(l.Type),
			Name:        l.Name,
			BaseX:       l.Base[0],
			BaseZ:       l.Base[1],
			TopX:        l.Top[0],
			TopZ:        l.Top[1],
			Speed:       l.Speed,
			TicketPrice: l.TicketPrice,
			Open:        l.Open,
			Chairs:      chairs,
			QueueIDs:    queueIDs,
		}
	}

	// Persist the full Guests master list so identity + career stats survive
	// the round-trip. Sim-scratch fields are only populated for
	// State==OnMountain rows; dormant rows serialise to a compact identity
	// stub.
	guests := make([]GuestData, len(w.Guests))
	for i, g := range w.Guests {
		gd := GuestData{
			ID:               g.ID,
			Name:             g.Name,
			Discipline:       uint8(g.Discipline),
			Skill:            uint8(g.Traits.Skill),
			LikesGlades:      g.Traits.LikesGlades,
			PrefersGroomed:   g.Traits.PrefersGroomed,
			VisitsPerSeason:  g.VisitsPerSeason,
			VisitsThisSeason: g.VisitsThisSeason,
			LifetimeVisits:   g.LifetimeVisits,
			LastScore:        g.LastScore,
			State:            uint8(g.State),
		}
		if !g.LastVisit.IsZero() {
			gd.LastVisitUnix = g.LastVisit.Unix()
		}
		if g.State == world.OnMountain {
			gd.Pos = [3]float32{g.Pos[0], g.Pos[1], g.Pos[2]}
			gd.Heading = g.Heading
			gd.Path = g.Path
			gd.PathIdx = g.PathIdx
			gd.Speed = g.Speed
			gd.TargetID = g.TargetID
			gd.OnLiftID = g.OnLiftID
			gd.Queued = g.Queued
			gd.Patience = g.Patience
			if !g.Plan.Done() {
				gd.PlanStep = g.Plan.Step
				gd.PlanSteps = make([]PlanActionData, len(g.Plan.Steps))
				for si, pa := range g.Plan.Steps {
					gd.PlanSteps[si] = PlanActionData{
						Kind:    uint8(pa.Kind),
						LiftID:  pa.LiftID,
						BldgID:  pa.BldgID,
						TrailID: pa.TrailID,
						Cost:    pa.Cost,
					}
				}
			}
		}
		guests[i] = gd
	}

	roadNodes := make([]RoadNodeData, len(w.RoadNodes))
	for i, n := range w.RoadNodes {
		roadNodes[i] = RoadNodeData{
			ID:   n.ID,
			X:    n.Pos[0],
			Z:    n.Pos[1],
			Kind: uint8(n.Kind),
		}
	}
	roadEdges := make([]RoadEdgeData, len(w.RoadEdges))
	for i, e := range w.RoadEdges {
		roadEdges[i] = RoadEdgeData{
			ID: e.ID,
			A:  e.A,
			B:  e.B,
		}
	}

	trails := make([]TrailData, len(w.Trails))
	for i, tr := range w.Trails {
		trails[i] = TrailData{
			ID:         tr.ID,
			Name:       tr.Name,
			Difficulty: uint8(tr.Difficulty),
			Groomed:    tr.Groomed,
			Cells:      tr.Cells,
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
		Trails:    trails,
		Guests:    guests,
		Snowcats:  snowcats,
		RoadNodes: roadNodes,
		RoadEdges: roadEdges,
		Cash:      w.Cash,
		History:   historyToData(w.History),
	}
}

// historyFromData rehydrates the History ring from saved chronological
// samples. Returns a freshly-allocated empty History when hd is nil so
// the sim immediately starts recording on the loaded world.
func historyFromData(hd *HistoryData) *world.History {
	h := world.NewHistory()
	if hd == nil {
		return h
	}
	h.ArrivalsToday = hd.ArrivalsToday
	h.DeparturesToday = hd.DeparturesToday
	for _, s := range hd.Samples {
		sample := world.DailySample{
			GuestsOnMountain: s.GuestsOnMountain,
			ArrivalsToday:    s.ArrivalsToday,
			DeparturesToday:  s.DeparturesToday,
			Cash:             s.Cash,
			Revenue:          s.Revenue,
			Costs:            s.Costs,
		}
		if s.DayUnix != 0 {
			sample.Day = time.Unix(s.DayUnix, 0).UTC()
		}
		h.Push(sample)
		// Push consumes (zeroes) ArrivalsToday/DeparturesToday on the
		// history struct; restore them after the last Push for the
		// caller's in-progress day.
	}
	h.ArrivalsToday = hd.ArrivalsToday
	h.DeparturesToday = hd.DeparturesToday
	h.RevenueToday = hd.RevenueToday
	return h
}

// historyToData captures the History ring in chronological order so the
// loader doesn't need to know about Head bookkeeping. Returns nil for
// nil input — msgpack then omits the field entirely.
func historyToData(h *world.History) *HistoryData {
	if h == nil {
		return nil
	}
	ordered := h.Ordered()
	samples := make([]DailySampleData, len(ordered))
	for i, s := range ordered {
		samples[i] = DailySampleData{
			GuestsOnMountain: s.GuestsOnMountain,
			ArrivalsToday:    s.ArrivalsToday,
			DeparturesToday:  s.DeparturesToday,
			Cash:             s.Cash,
			Revenue:          s.Revenue,
			Costs:            s.Costs,
		}
		if !s.Day.IsZero() {
			samples[i].DayUnix = s.Day.Unix()
		}
	}
	return &HistoryData{
		Samples:         samples,
		ArrivalsToday:   h.ArrivalsToday,
		DeparturesToday: h.DeparturesToday,
		RevenueToday:    h.RevenueToday,
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
				t.Cells[x][z].GroundElevation = c.Ground
				t.Cells[x][z].SnowAccumulation = c.Snow
				t.Cells[x][z].Grooming = c.Grooming
				t.Cells[x][z].Packed = c.Packed
				t.Cells[x][z].Ice = c.Ice
				t.Cells[x][z].MogulSize = c.MogulSize
				t.Cells[x][z].TreeDensity = c.TreeDensity
			}
			idx++
		}
	}

	w := world.NewWorld(t)
	if data.Cash != 0 {
		w.Cash = data.Cash
	}

	// Restore objects (no cross-references, fresh IDs are fine).
	for _, od := range data.Objects {
		obj := w.PlaceObject(world.ObjectType(od.Type), od.X, od.Z)
		obj.Rotation = od.Rotation
	}

	// Restore buildings, preserving IDs so agent.TargetID references stay
	// valid. Old saves without an `id` field fall back to a fresh ID.
	for _, bd := range data.Buildings {
		b := w.PlaceBuildingType(world.BuildingType(bd.Type), bd.X, bd.Z)
		// PlaceBuildingType auto-spawns one cat for sheds; clear that
		// so we can restore the saved fleet (and route) verbatim
		// instead of double-spawning.
		if b.Type == world.BuildingShed {
			w.RemoveSnowcatsOwnedBy(b.ID)
		}
		if bd.ID != 0 {
			b.ID = bd.ID
		}
		b.Rotation = bd.Rotation
		// Shed-only state. Cats defaults to zero on older saves; the shed
		// loads with one cat (the default from PlaceBuildingType) until
		// the player adjusts via the popup.
		if b.Type == world.BuildingShed && bd.Cats > 0 {
			b.Cats = bd.Cats
		}
		// Parking-only state. MaxCars/CurrentCars default to zero on
		// older saves; the placement defaults from PlaceBuildingType
		// already populated MaxCars to a reasonable value above. The
		// driveway node is restored via the road-node pass below; we
		// just relink the building's pointer here. PlaceBuildingType
		// doesn't auto-create driveways, so there's no conflict to
		// undo first.
		if b.Type == world.BuildingParking {
			if bd.MaxCars > 0 {
				b.MaxCars = bd.MaxCars
			}
			b.CurrentCars = bd.CurrentCars
			b.DrivewayNodeIDs = bd.DrivewayNodeIDs
		}
	}

	// Restore snowcats. Each cat's ShedID must already resolve via the
	// just-restored buildings; otherwise we drop the cat as orphaned.
	shedByID := make(map[uint64]*world.Building)
	for _, b := range w.Buildings {
		if b.Type == world.BuildingShed {
			shedByID[b.ID] = b
		}
	}
	for _, cd := range data.Snowcats {
		shed := shedByID[cd.ShedID]
		if shed == nil {
			continue
		}
		cat := w.SpawnSnowcat(shed)
		if cd.ID != 0 {
			cat.ID = cd.ID
		}
		cat.Pos = mgl32.Vec3{cd.Pos[0], cd.Pos[1], cd.Pos[2]}
		cat.Heading = cd.Heading
		cat.TargetCell = cd.TargetCell
	}

	// Restore lifts. Chair count is computed from cable length so it's
	// stable across save/load; we still copy progress + passenger refs
	// from the saved chair list. Queue IDs are resolved after agents
	// load below.
	for _, ld := range data.Lifts {
		lift := w.PlaceLift(world.LiftType(ld.Type), ld.BaseX, ld.BaseZ, ld.TopX, ld.TopZ)
		if ld.ID != 0 {
			lift.ID = ld.ID
		}
		// Only adopt the saved name when it's non-empty; old saves that
		// predate auto-naming leave Name blank, so we keep the LiftN
		// default that PlaceLift just assigned.
		if ld.Name != "" {
			lift.Name = ld.Name
		}
		if ld.Speed >= 0.1 {
			lift.Speed = ld.Speed
		}
		if ld.TicketPrice > 0 {
			lift.TicketPrice = ld.TicketPrice
		}
		lift.Open = ld.Open
		// Restore chair Progress where the saved length matches; if the
		// chair count differs (e.g. a code change), keep the freshly
		// initialised even-spacing for the unmatched chairs.
		for ci := range lift.Chairs {
			if ci < len(ld.Chairs) {
				lift.Chairs[ci].Progress = ld.Chairs[ci].Progress
			}
		}
	}

	// Restore guests. Every row in data.Guests rehydrates into a *Guest in
	// w.Guests (the master catchment, including dormant entries); rows with
	// State==OnMountain also get a pointer into w.OnMountain so the sim
	// ticks them. IDs are preserved so chair / queue references resolve.
	for _, gd := range data.Guests {
		var id uint64
		if gd.ID != 0 {
			id = gd.ID
		} else {
			id = w.NextID()
		}
		traits := ai.TraitsFor(ai.SkillLevel(gd.Skill))
		traits.LikesGlades = gd.LikesGlades
		traits.PrefersGroomed = gd.PrefersGroomed
		g := &world.Guest{
			ID:               id,
			Name:             gd.Name,
			Discipline:       world.Discipline(gd.Discipline),
			Traits:           traits,
			VisitsPerSeason:  gd.VisitsPerSeason,
			VisitsThisSeason: gd.VisitsThisSeason,
			LifetimeVisits:   gd.LifetimeVisits,
			LastScore:        gd.LastScore,
			State:            world.GuestState(gd.State),
		}
		if gd.LastVisitUnix != 0 {
			g.LastVisit = time.Unix(gd.LastVisitUnix, 0).UTC()
		}
		if g.State == world.OnMountain {
			g.Pos = mgl32.Vec3{gd.Pos[0], gd.Pos[1], gd.Pos[2]}
			g.Heading = gd.Heading
			g.Path = gd.Path
			g.PathIdx = gd.PathIdx
			g.Speed = gd.Speed
			g.TargetID = gd.TargetID
			g.OnLiftID = gd.OnLiftID
			g.Queued = gd.Queued
			patience := gd.Patience
			if patience <= 0 {
				patience = 1.0
			}
			g.Patience = patience
			g.Balance = 1.0
			if len(gd.PlanSteps) > 0 {
				steps := make([]ai.PlanAction, len(gd.PlanSteps))
				for si, pd := range gd.PlanSteps {
					steps[si] = ai.PlanAction{
						Kind:    ai.PlanActionKind(pd.Kind),
						LiftID:  pd.LiftID,
						BldgID:  pd.BldgID,
						TrailID: pd.TrailID,
						Cost:    pd.Cost,
					}
				}
				g.Plan.Steps = steps
				g.Plan.Step = gd.PlanStep
			}
			w.OnMountain = append(w.OnMountain, g)
		}
		w.Guests = append(w.Guests, g)
	}

	// Build an ID lookup so we can resolve chair-passenger and queue
	// references back to live *Guest pointers.
	guestByID := make(map[uint64]*world.Guest, len(w.OnMountain))
	for _, a := range w.OnMountain {
		guestByID[a.ID] = a
	}

	for li, ld := range data.Lifts {
		lift := w.Lifts[li]
		// Chairs: re-link passengers by ID. Drop refs that don't resolve.
		for ci := range lift.Chairs {
			if ci >= len(ld.Chairs) {
				break
			}
			for pi, pid := range ld.Chairs[ci].PassengerIDs {
				if pid == 0 || pi >= len(lift.Chairs[ci].Passengers) {
					continue
				}
				if a := guestByID[pid]; a != nil {
					lift.Chairs[ci].Passengers[pi] = a
				}
			}
		}
		// Queue: rebuild in saved order, skipping unresolved IDs.
		for _, qid := range ld.QueueIDs {
			if a := guestByID[qid]; a != nil {
				lift.Queue = append(lift.Queue, a)
			}
		}
	}

	// Restore road graph. Nodes first so edges can reference them by ID.
	// Old saves without road data leave both slices empty — there's no
	// implicit road network to fall back to.
	for _, nd := range data.RoadNodes {
		pos := mgl32.Vec2{nd.X, nd.Z}
		n := w.AddRoadNode(pos, world.RoadNodeKind(nd.Kind))
		if nd.ID != 0 {
			n.ID = nd.ID
		}
	}
	for _, ed := range data.RoadEdges {
		e := w.AddRoadEdge(ed.A, ed.B)
		if ed.ID != 0 {
			e.ID = ed.ID
		}
	}

	// Restore trails. TrailGraph is derived on load rather than persisted.
	for _, td := range data.Trails {
		t := w.PlaceTrail(td.Name, world.TerrainDifficulty(td.Difficulty))
		if td.ID != 0 {
			t.ID = td.ID
		}
		t.Groomed = td.Groomed
		t.Cells = td.Cells
	}
	if len(w.Trails) > 0 {
		w.RebuildTrailGraph()
	}

	// Validate parking driveways now that road nodes are in place.
	// EnsureParkingDriveway is idempotent — a parking lot whose
	// DrivewayNodeID resolves to an existing node is left alone; one
	// with a missing or zero ID gets a fresh driveway. Covers older
	// saves that predate the driveway field and any corrupted graphs.
	for _, b := range w.Buildings {
		if b.Type == world.BuildingParking {
			w.EnsureParkingDriveway(b)
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
	for _, a := range w.OnMountain {
		if a.ID > maxID {
			maxID = a.ID
		}
	}
	for _, c := range w.Snowcats {
		if c.ID > maxID {
			maxID = c.ID
		}
	}
	for _, n := range w.RoadNodes {
		if n.ID > maxID {
			maxID = n.ID
		}
	}
	for _, e := range w.RoadEdges {
		if e.ID > maxID {
			maxID = e.ID
		}
	}
	for _, tr := range w.Trails {
		if tr.ID > maxID {
			maxID = tr.ID
		}
	}
	w.SetMinNextID(maxID)

	// Fresh scenarios (and any pre-Guests save) land here with an empty
	// catchment. Seed a default 10k pool so the demand poll has someone
	// to draw from. Post-rewrite saves write their Guests slice and skip
	// this branch.
	if len(w.Guests) == 0 {
		world.SeedGuests(w, rand.New(rand.NewSource(time.Now().UnixNano())), world.DefaultGuestPoolSize)
	}

	// Rehydrate the history ring. Absent in the save → allocate an
	// empty *History so the sim starts recording immediately.
	w.History = historyFromData(data.History)

	return w
}
