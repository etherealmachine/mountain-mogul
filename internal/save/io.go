package save

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math"
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

// startDateLayout is the ScenarioData.StartDate encoding.
const startDateLayout = "2006-01-02"

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
	return saveWorld(path, w, cam, false)
}

// SaveStarterScenario writes the world as a starter scenario: the same
// format as SaveScenario, minus today's in-flight state. Guests on the
// mountain go home, queues and chairs are emptied, parking lots are
// empty, and snowcats and patrollers are parked at their buildings, so
// a New Game from the file starts clean. Everything the player built
// (and the snow, history, cash and camera) is kept.
func SaveStarterScenario(path string, w *world.World, cam *CameraData) error {
	return saveWorld(path, w, cam, true)
}

func saveWorld(path string, w *world.World, cam *CameraData, forScenario bool) error {
	data := worldToData(w, forScenario)
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
	data, err := ReadScenarioData(path)
	if err != nil {
		return nil, nil, err
	}
	return dataToWorld(data), data.Camera, nil
}

// ReadScenarioData decodes a `.save` file without building a world, for
// tools that patch one field and write it back with WriteScenarioData.
func ReadScenarioData(path string) (ScenarioData, error) {
	var data ScenarioData
	raw, err := os.ReadFile(path)
	if err != nil {
		return data, err
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return data, fmt.Errorf("save %q: %w", path, err)
	}
	defer gz.Close()
	dec := msgpack.NewDecoder(gz)
	dec.SetCustomStructTag("json")
	if err := dec.Decode(&data); err != nil && err != io.EOF {
		return data, fmt.Errorf("save %q: %w", path, err)
	}
	return data, nil
}

// legacyScenarioName is the Name every save carried before scenarios had
// real names.
const legacyScenarioName = "scenario"

func scenarioInfoOf(data ScenarioData) world.ScenarioInfo {
	name := data.Name
	if name == legacyScenarioName {
		name = ""
	}
	return world.ScenarioInfo{
		Name:        name,
		Description: data.Description,
		Location:    data.Location,
		Difficulty:  data.Difficulty,
		Order:       data.Order,
		Tutorial:    data.Tutorial,
		File:        data.ScenarioFile,
	}
}

// ReadScenarioInfo reads just the scenario's name, description, and
// campaign placing from a save, without decoding the terrain. Lists of
// saves and scenarios call it once per file.
func ReadScenarioInfo(path string) (world.ScenarioInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return world.ScenarioInfo{}, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return world.ScenarioInfo{}, fmt.Errorf("save %q: %w", path, err)
	}
	defer gz.Close()
	dec := msgpack.NewDecoder(gz)
	n, err := dec.DecodeMapLen()
	if err != nil {
		return world.ScenarioInfo{}, fmt.Errorf("save %q: %w", path, err)
	}
	var data ScenarioData
	for i := 0; i < n; i++ {
		key, err := dec.DecodeString()
		if err != nil {
			return world.ScenarioInfo{}, fmt.Errorf("save %q: %w", path, err)
		}
		var dst any
		switch key {
		case "name":
			dst = &data.Name
		case "description":
			dst = &data.Description
		case "location":
			dst = &data.Location
		case "difficulty":
			dst = &data.Difficulty
		case "order":
			dst = &data.Order
		case "tutorial":
			dst = &data.Tutorial
		case "cells":
			// The info fields are written before the cells.
			return scenarioInfoOf(data), nil
		}
		if dst == nil {
			err = dec.Skip()
		} else {
			err = dec.Decode(dst)
		}
		if err != nil {
			return world.ScenarioInfo{}, fmt.Errorf("save %q: %w", path, err)
		}
	}
	return scenarioInfoOf(data), nil
}

// worldToData snapshots w. forScenario drops today's in-flight state
// (see SaveStarterScenario).
func worldToData(w *world.World, forScenario bool) ScenarioData {
	t := w.Terrain
	cells := make([]CellData, 0, t.Width*t.Height)
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			c := t.Cells[x][z]
			var layers []LayerData
			if c.Base > 0 {
				layers = append(layers, LayerData{A: c.Base, K: uint8(world.KindBase)})
			}
			if c.Top.Accumulation > 0 {
				layers = append(layers, LayerData{A: c.Top.Accumulation, K: uint8(c.Top.Kind)})
			}
			cells = append(cells, CellData{
				Ground:       c.GroundElevation,
				Layers:       layers,
				Grooming:     c.Grooming,
				MogulSize:    c.MogulSize,
				SkierTraffic: c.SkierTraffic,
			})
		}
	}

	trees := make([]float32, 0, 2*t.TotalTrees())
	t.ForEachStoredTree(func(tr world.Tree) {
		trees = append(trees, tr.X, tr.Z)
	})

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
			MaxCars:         b.MaxCars,
			DrivewayNodeIDs: b.DrivewayNodeIDs,
			DriveEdge:       b.DriveEdge,
			LotSize:         [2]float32{b.LotSize[0], b.LotSize[1]},
			SnowGunEnabled:  b.SnowGunEnabled,
			StyleSeed:       b.StyleSeed,
			MealPrice:       b.MealPrice,
			Quality:         &b.Quality,
			Rental:          b.RentalPrice,
			DrinkPrice:      b.DrinkPrice,
			FreeWater:       b.FreeWater,
		}
		if b.IsShell() {
			buildings[i].OriginX, buildings[i].OriginZ = b.Origin[0], b.Origin[1]
			buildings[i].Kind, buildings[i].Storeys = uint8(b.Kind), b.Storeys
			buildings[i].Tiles = saveTiles(b)
			buildings[i].FloorY, buildings[i].FloorSet = b.FloorY, b.FloorSet
		}
	}

	buildingByID := make(map[uint64]*world.Building, len(w.Buildings))
	for _, b := range w.Buildings {
		buildingByID[b.ID] = b
	}

	snowcats := make([]SnowcatData, len(w.Snowcats))
	for i, c := range w.Snowcats {
		snowcats[i] = SnowcatData{
			ID:      c.ID,
			ShedID:  c.ShedID,
			Pos:     [3]float32{c.Pos[0], c.Pos[1], c.Pos[2]},
			Heading: c.Heading,
			Status:  uint8(c.Status),
		}
		if shed := buildingByID[c.ShedID]; forScenario && shed != nil {
			p := w.SnowcatParkPos(shed)
			snowcats[i].Pos = [3]float32{p[0], p[1], p[2]}
			snowcats[i].Heading = 0
		}
	}

	snowmobiles := make([]SnowmobileData, len(w.Snowmobiles))
	for i, m := range w.Snowmobiles {
		snowmobiles[i] = SnowmobileData{ID: m.ID, GarageID: m.GarageID, Pos: [3]float32{m.Pos[0], m.Pos[1], m.Pos[2]}, Heading: m.Heading, InGarage: m.InGarage, TakenBy: m.TakenBy}
		if g := buildingByID[m.GarageID]; forScenario && g != nil {
			p := w.GarageSpot(g)
			snowmobiles[i] = SnowmobileData{ID: m.ID, GarageID: m.GarageID, Pos: [3]float32{p[0], p[1], p[2]}, InGarage: true}
		}
	}

	patrollers := make([]PatrollerData, len(w.Patrollers))
	for i, p := range w.Patrollers {
		patrollers[i] = PatrollerData{
			ID:         p.ID,
			HutID:      p.HutID,
			Pos:        [3]float32{p.Pos[0], p.Pos[1], p.Pos[2]},
			Heading:    p.Heading,
			State:      uint8(p.State),
			Snowmobile: p.SnowmobileID,
			Target:     p.TargetGuestID,
			TargetPos:  [3]float32{p.TargetPos[0], p.TargetPos[1], p.TargetPos[2]},
			Timer:      p.ActionTimer,
			OnSkis:     p.OnSkis,
			Lift:       p.LiftID,
			LiftT:      p.LiftProgress,
		}
		if hut := buildingByID[p.HutID]; forScenario && hut != nil {
			pos := w.PatrollerHutPos(hut)
			patrollers[i] = PatrollerData{ID: p.ID, HutID: p.HutID, Pos: [3]float32{pos[0], pos[1], pos[2]}, State: uint8(world.PatrollerOffDuty)}
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
				if pax != nil && !forScenario {
					cd.PassengerIDs[pi] = pax.ID
				}
			}
			chairs[ci] = cd
		}
		queueIDs := make([]uint64, 0, len(l.Queue))
		for _, a := range l.Queue {
			if a != nil && !forScenario {
				queueIDs = append(queueIDs, a.ID)
			}
		}
		ld := LiftData{
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
			LeftLines:   l.QueueConfig.LeftLines,
			RightLines:  l.QueueConfig.RightLines,
			SingleRider: l.QueueConfig.SingleRider,

			LineAttendant: l.Staff.LineAttendant,
		}
		if len(l.Lines) > 0 && !forScenario {
			ld.LineQueueIDs = make([][]uint64, len(l.Lines))
			for li, line := range l.Lines {
				ids := make([]uint64, 0, len(line.Guests))
				for _, g := range line.Guests {
					if g != nil {
						ids = append(ids, g.ID)
					}
				}
				ld.LineQueueIDs[li] = ids
			}
		}
		if l.IsHeli() && l.HeliState != nil && !forScenario {
			ld.HeliPhase = uint8(l.HeliState.Phase)
			ld.HeliProgress = l.HeliState.Progress
		}
		lifts[i] = ld
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
			Skill:            g.Traits.Skill,
			Tastes:           append([]float32(nil), g.Traits.Tastes[:]...),
			ArrivalOffset:    &g.ArrivalOffset,
			Leaving:          leavingToData(g.Leaving),
			VisitsPerSeason:  g.VisitsPerSeason,
			HomeEntry:        g.HomeEntryID,
			Group:            g.GroupID,
			VisitsThisSeason: g.VisitsThisSeason,
			LifetimeVisits:   g.LifetimeVisits,
			LastStars:        g.LastStars,
			State:            uint8(g.State),
		}
		if forScenario {
			gd.State = uint8(world.AtHome)
		} else {
			gd.CarID, gd.CarLot = g.CarID, g.CarLot
		}
		if !g.LastVisit.IsZero() {
			gd.LastVisitUnix = g.LastVisit.Unix()
		}
		gd.SeasonPassExpiry = g.SeasonPassExpiry
		gd.HasSeasonPass = g.HasSeasonPass
		if gd.State == uint8(world.OnMountain) {
			gd.DayTicketDue = g.DayTicketDue
			gd.DayTicketPaid = g.DayTicketPaid
			gd.HasDayTicket = g.HasDayTicket
			gd.RemainingBudget = g.RemainingBudget
			gd.Pos = [3]float32{g.Pos[0], g.Pos[1], g.Pos[2]}
			gd.Heading = g.Heading
			gd.Path = g.Path
			gd.PathIdx = g.PathIdx
			gd.Speed = g.Speed
			gd.TargetID = g.TargetID
			gd.OnLiftID = g.OnLiftID
			gd.Queued = g.Queued
			gd.Patience = g.Patience
			gd.Energy = g.Energy
			gd.Hunger = g.Hunger
			gd.Thirst = g.Thirst
			gd.SkisOn, gd.NeedsGear = g.SkisOn, g.NeedsGear
			if g.Stash.Out {
				gd.Skis = &SkisData{Rack: g.Stash.RackID, Slot: g.Stash.Slot, Pos: [2]float32{g.Stash.Pos[0], g.Stash.Pos[1]}, Yaw: g.Stash.Yaw, Lying: g.Stash.Lying}
			}
			for _, m := range g.Moments {
				gd.Moments = append(gd.Moments, MomentData{Kind: uint8(m.Kind), N: m.N, Context: m.Context})
			}
			gd.QualitySum, gd.QualityUses = g.QualitySum, g.QualityUses
			gd.TrailTally, gd.LiftTally = tallyToData(g.TrailTally), tallyToData(g.LiftTally)
			if !g.Plan.Done() {
				gd.PlanStep = g.Plan.Step
				gd.PlanSteps = make([]PlanActionData, len(g.Plan.Steps))
				for si, pa := range g.Plan.Steps {
					gd.PlanSteps[si] = PlanActionData{
						Kind:    uint8(pa.Kind),
						LiftID:  pa.LiftID,
						BldgID:  pa.BldgID,
						TrailID: pa.TrailID,
						Via:     pa.Via,
						Use:     uint8(pa.Use),
						Cost:    pa.Cost,
					}
				}
			}
		}
		guests[i] = gd
	}

	var cars []CarData
	if !forScenario {
		for _, c := range w.Cars {
			cd := CarData{
				ID: c.ID, Kind: uint8(c.Kind), Roof: uint8(c.Roof),
				Entry: c.Entry, Lot: c.Lot, Stall: c.Stall,
				State: uint8(c.State), Route: c.Route, Leg: c.Leg, D: c.D,
				Speed: c.Speed, Pos: [2]float32{c.Pos[0], c.Pos[1]},
				Heading: c.Heading, InLot: c.InLot,
			}
			for _, g := range c.Guests {
				cd.Guests = append(cd.Guests, g.ID)
			}
			cars = append(cars, cd)
		}
	}

	roadNodes := make([]RoadNodeData, len(w.RoadNodes))
	for i, n := range w.RoadNodes {
		roadNodes[i] = RoadNodeData{
			ID:   n.ID,
			X:    n.Pos[0],
			Z:    n.Pos[1],
			Kind: uint8(n.Kind),
			Name: n.Name,
			Pool: n.Pool,
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

	var footpaths []FootpathData
	for _, f := range w.Footpaths {
		fd := FootpathData{ID: f.ID}
		for _, n := range f.Nodes {
			fd.Nodes = append(fd.Nodes, [3]float32{n.Pos[0], n.Pos[1], n.Width})
		}
		footpaths = append(footpaths, fd)
	}
	var ropes []RopeData
	for _, r := range w.Ropes {
		rd := RopeData{ID: r.ID}
		for _, n := range r.Nodes {
			rd.Nodes = append(rd.Nodes, [2]float32{n[0], n[1]})
		}
		ropes = append(ropes, rd)
	}
	trails := make([]TrailData, len(w.Trails))
	for i, tr := range w.Trails {
		trails[i] = TrailData{
			ID:         tr.ID,
			Name:       tr.Name,
			Kind:       uint8(tr.Kind),
			Difficulty: uint8(tr.Difficulty),
			Groomed:    tr.Groomed,
			Start:      trailEndToData(tr.Start),
			End:        trailEndToData(tr.End),
		}
		for _, n := range tr.Nodes {
			trails[i].Nodes = append(trails[i].Nodes, [3]float32{n.Pos[0], n.Pos[1], n.Width})
		}
		for _, p := range tr.Outline {
			trails[i].Outline = append(trails[i].Outline, [2]float32{p[0], p[1]})
		}
	}

	parcels := make([]ParcelData, len(w.Parcels))
	for i, p := range w.Parcels {
		parcels[i] = ParcelData{
			ID:    p.ID,
			Name:  p.Name,
			State: uint8(p.State),
			Price: p.Price,
			Cells: p.Cells,
		}
	}

	simTime := w.SimTime
	if forScenario {
		// New games from a starter scenario open on its day's morning.
		day := math.Floor(simTime / world.SecondsPerSimDay)
		simTime = day*world.SecondsPerSimDay + world.NewGameStartHour*world.SimSecondsPerHour
	}
	return ScenarioData{
		Name:         w.Scenario.Name,
		Description:  w.Scenario.Description,
		Location:     w.Scenario.Location,
		Difficulty:   w.Scenario.Difficulty,
		Order:        w.Scenario.Order,
		Tutorial:     w.Scenario.Tutorial,
		ScenarioFile: w.Scenario.File,

		Seed:         w.Seed,
		SimTime:      simTime,
		DaySec:       world.SecondsPerSimDay,
		OpenHour:     w.OpenHour,
		CloseHour:    w.CloseHour,
		StartDate:    w.StartDate.Format(startDateLayout),
		GroupMix:     groupMixData(w.GroupMix),
		Width:        t.Width,
		Height:       t.Height,
		Cells:        cells,
		Objects:      objects,
		Trees:        trees,
		Groom:        groomPixels(t),
		Moguls:       mogulPixels(t),
		Detail:       detailBytes(t),
		Material:     materialBytes(t),
		LakeOf:       t.LakeOf,
		LakeDepth:    lakeDepthBytes(t.LakeDepth),
		Lakes:        lakesToData(w.Lakes),
		Geo:          geoToData(w.Geo),
		BaseAltitude: w.BaseAltitude,
		TimeZone:     w.TimeZone,
		Climate:      climateToData(w.Climate),
		TerrainBase:  terrainBaseToData(w.TerrainBase),
		Buildings:    buildings,
		Lifts:        lifts,
		Trails:       trails,
		Footpaths:    footpaths,
		Ropes:        ropes,
		Guests:       guests,
		Snowcats:     snowcats,
		Snowmobiles:  snowmobiles,
		Patrollers:   patrollers,
		RoadNodes:    roadNodes,
		Cars:         cars,
		RoadEdges:    roadEdges,
		Parcels:      parcels,
		SkiArea:      skiAreaData(w),
		Cash:         &w.Cash,
		DayTicket:    &w.DayTicketPrice,
		SeasonPass:   &w.SeasonPassPrice,
		CreditRate:   &w.CreditRate,
		Parking:      w.ParkingPrice,
		ResortOpen:   w.ResortOpen,
		Stars:        &w.Rating,
		Goals:        goalsToData(w.Goals),
		Rules:        w.Rules,
		GoalProgress: progressToData(w.GoalProgress),
		Outcome:      uint8(w.Outcome),
		OutcomeDay:   w.OutcomeDay,

		CreditLimit:     &w.CreditLimit,
		AccruedInterest: w.AccruedInterest,
		DaysBelowFloor:  w.DaysBelowFloor,
		Bankrupt:        w.Bankrupt,
		History:         historyToData(w.History),
		Events:          eventsToData(&w.Events),
	}
}

// eventsToData captures the event feed oldest-first. Returns nil for an
// empty feed so msgpack omits the field.
// isLegacySkillEnum reports whether guests were saved when Skill was a
// Beginner/Intermediate/Advanced enum (0/1/2) rather than a [0, 1]
// float. A value of 2 can only come from the enum; every value must be
// a whole tier number too.
func isLegacySkillEnum(guests []GuestData) bool {
	sawTwo := false
	for _, gd := range guests {
		switch gd.Skill {
		case 0, 1:
		case 2:
			sawTwo = true
		default:
			return false
		}
	}
	return sawTwo
}

func eventsToData(l *world.EventLog) []EventData {
	if l.Len() == 0 {
		return nil
	}
	out := make([]EventData, l.Len())
	for i := range out {
		e := l.At(i)
		out[i] = EventData{
			Kind:     uint8(e.Kind),
			SimTime:  e.SimTime,
			Message:  e.Message,
			HasPos:   e.HasPos,
			X:        e.Pos[0],
			Z:        e.Pos[1],
			EntityID: e.EntityID,
		}
	}
	return out
}

// eventsFromData replays saved events into l in chronological order.
func eventsFromData(l *world.EventLog, data []EventData) {
	for _, e := range data {
		l.Push(world.Event{
			Kind:     world.EventKind(e.Kind),
			SimTime:  e.SimTime,
			Message:  e.Message,
			HasPos:   e.HasPos,
			Pos:      mgl32.Vec2{e.X, e.Z},
			EntityID: e.EntityID,
		})
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
			Open:             s.Open,
			Rating:           s.Rating,
			Falls:            s.Falls,
			Reviews:          reviewTallyFromData(s.Reviews),
		}
		copy(sample.DepartReasons[:], s.DepartReasons)
		copy(sample.RevenueByKind[:], s.RevenueByKind)
		copy(sample.CostsByKind[:], s.CostsByKind)
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
	copy(h.RevenueByKindToday[:], hd.RevenueByKind)
	copy(h.DepartReasonsToday[:], hd.DepartReasons)
	h.ReviewsToday = reviewTallyFromData(hd.Reviews)
	for _, f := range hd.Falls {
		h.FallsToday = append(h.FallsToday, world.FallRecord{X: f.X, Z: f.Z, TrailID: f.TrailID, LiftID: f.LiftID})
	}
	return h
}

// historyToData captures the History ring in chronological order so the
// loader doesn't need to know about Head bookkeeping. Returns nil for
// nil input — msgpack then omits the field entirely.
func historyToData(h *world.History) *HistoryData {
	if h == nil {
		return nil
	}
	var falls []FallData
	for _, f := range h.FallsToday {
		falls = append(falls, FallData{X: f.X, Z: f.Z, TrailID: f.TrailID, LiftID: f.LiftID})
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
			RevenueByKind:    intsOrNil(s.RevenueByKind[:]),
			CostsByKind:      intsOrNil(s.CostsByKind[:]),
			Open:             s.Open,
			Rating:           s.Rating,
			Falls:            s.Falls,
			Reviews:          reviewTallyToData(s.Reviews),
			DepartReasons:    intsOrNil(s.DepartReasons[:]),
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
		RevenueByKind:   intsOrNil(h.RevenueByKindToday[:]),
		Falls:           falls,
		Reviews:         reviewTallyToData(h.ReviewsToday),
		DepartReasons:   intsOrNil(h.DepartReasonsToday[:]),
	}
}

// reviewTallyToData saves a day's reviews, nil when there were none.
func reviewTallyToData(t world.ReviewTally) *ReviewTallyData {
	if t.N == 0 {
		return nil
	}
	return &ReviewTallyData{
		N: t.N, Stars: t.Stars, Levels: intsOrNil(t.Levels[:]), Why: intsOrNil(t.Why[:]),
		NothingSpecial: t.NothingSpecial, NoRuns: t.NoRuns, Good: t.Good,
	}
}

// reviewTallyFromData restores a day's reviews.
func reviewTallyFromData(d *ReviewTallyData) world.ReviewTally {
	if d == nil {
		return world.ReviewTally{}
	}
	t := world.ReviewTally{N: d.N, Stars: d.Stars, NothingSpecial: d.NothingSpecial, NoRuns: d.NoRuns, Good: d.Good}
	copy(t.Levels[:], d.Levels)
	copy(t.Why[:], d.Why)
	return t
}

// intsOrNil copies v, or returns nil when every entry is zero so the
// field is omitted from the save.
func intsOrNil(v []int) []int {
	for _, x := range v {
		if x != 0 {
			return append([]int(nil), v...)
		}
	}
	return nil
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
				t.Cells[x][z].Grooming = c.Grooming
				t.Cells[x][z].MogulSize = c.MogulSize
				t.Cells[x][z].SkierTraffic = c.SkierTraffic
				if c.TreeDensity > 0 {
					t.SetCellTreesFromDensity(x, z, c.TreeDensity)
				}
				// ls[]=[] → bare ground; ls=[1] → Top only; ls=[2] → Base+Top.
				// Old saves with >2 layers: treat last as Top, second-to-last as Base.
				switch n := len(c.Layers); n {
				case 0:
					t.Cells[x][z].Base = 0
					t.Cells[x][z].Top = world.SnowLayer{}
				case 1:
					t.Cells[x][z].Base = 0
					t.Cells[x][z].Top = world.SnowLayer{Accumulation: c.Layers[0].A, Kind: world.SnowKind(c.Layers[0].K)}
				default:
					t.Cells[x][z].Base = c.Layers[n-2].A
					top := c.Layers[n-1]
					t.Cells[x][z].Top = world.SnowLayer{Accumulation: top.A, Kind: world.SnowKind(top.K)}
				}
			}
			idx++
		}
	}
	for i := 0; i+1 < len(data.Trees); i += 2 {
		t.AddTree(world.Tree{X: data.Trees[i], Z: data.Trees[i+1]})
	}
	t.RecomputeSlopes()
	t.LoadMoguls(data.Moguls)
	if !t.Groom.Load(data.Groom) {
		t.RestampGroomFromCells()
	}
	if len(data.Detail) > 0 {
		t.Detail = world.LoadTerrainDetail(t.Width, t.Height, data.Detail)
	}
	if len(data.Material) > 0 {
		t.Material = world.LoadTerrainMaterial(t.Width, t.Height, data.Material)
	}

	if len(data.LakeOf) == t.Width*t.Height && len(data.LakeDepth) == len(data.LakeOf) {
		t.LakeOf = data.LakeOf
		t.LakeDepth = make([]float32, len(data.LakeDepth))
		for k, dm := range data.LakeDepth {
			t.LakeDepth[k] = float32(dm) / 10
		}
	}

	w := world.NewWorld(t)
	w.Seed = data.Seed
	for _, l := range data.Lakes {
		w.Lakes = append(w.Lakes, world.Lake{Name: l.Name, Altitude: l.Altitude, AreaHa: l.AreaHa, MaxDepth: l.MaxDepth, Frost: l.Frost, Thaw: l.Thaw})
	}
	if data.Cash != nil {
		w.Cash = *data.Cash
	}
	if data.DayTicket != nil {
		w.DayTicketPrice = *data.DayTicket
	}
	if data.SeasonPass != nil {
		w.SeasonPassPrice = *data.SeasonPass
	}
	if data.CreditRate != nil {
		w.CreditRate = *data.CreditRate
	}
	w.ParkingPrice = data.Parking
	w.ResortOpen = data.ResortOpen
	if data.Stars != nil {
		w.Rating = *data.Stars
	}
	w.Rules = data.Rules
	for _, g := range data.Goals {
		w.Goals = append(w.Goals, world.Goal{Kind: world.GoalKind(g.Kind), Target: g.Target, Days: g.Days, Season: g.Season, Bonus: g.Bonus})
	}
	for _, p := range data.GoalProgress {
		w.GoalProgress = append(w.GoalProgress, world.GoalProgress{Met: p.Met, MetDay: p.MetDay, Streak: p.Streak, Best: p.Best, Failed: p.Failed})
	}
	w.Outcome, w.OutcomeDay = world.Outcome(data.Outcome), data.OutcomeDay
	// Progress follows the goals: an edited scenario's goal list wins.
	if len(w.GoalProgress) != len(w.Goals) {
		w.GoalProgress = make([]world.GoalProgress, len(w.Goals))
		w.Outcome, w.OutcomeDay = world.Playing, 0
	}
	if data.CreditLimit != nil {
		w.CreditLimit = *data.CreditLimit
	}
	w.AccruedInterest = data.AccruedInterest
	w.DaysBelowFloor = data.DaysBelowFloor
	w.Bankrupt = data.Bankrupt

	// Restore objects (no cross-references, fresh IDs are fine). Lone
	// trees from older saves join the stored trees at their cell centre.
	for _, od := range data.Objects {
		if world.ObjectType(od.Type) == world.ObjTree {
			t.AddTree(world.Tree{
				X: (float32(od.X) + 0.5) * world.CellSize,
				Z: (float32(od.Z) + 0.5) * world.CellSize,
			})
			continue
		}
		obj := w.PlaceObject(world.ObjectType(od.Type), od.X, od.Z)
		obj.Rotation = od.Rotation
	}

	// Restore buildings, preserving IDs so agent.TargetID references stay
	// valid. Old saves without an `id` field fall back to a fresh ID.
	for _, bd := range data.Buildings {
		// Lots and service buildings saved in older forms (painted lot
		// cells, point-placed lodges) are dropped, not converted, and so
		// are equipment sheds and patrol huts with their cats and
		// patrollers.
		switch world.BuildingType(bd.Type) {
		case world.BuildingParking:
			if bd.LotSize[0] <= 0 || bd.LotSize[1] <= 0 {
				continue
			}
		case world.BuildingLodge:
			if len(bd.Tiles) == 0 {
				continue
			}
		case world.BuildingBar, world.BuildingTicketOffice:
			continue
		case world.BuildingShed, world.BuildingPatrolHut:
			continue // removed; they return as building services
		}
		var b *world.Building
		switch world.BuildingType(bd.Type) {
		case world.BuildingLodge:
			b = loadServiceBuilding(w, bd)
		default:
			b = w.PlaceBuildingType(world.BuildingType(bd.Type), bd.X, bd.Z)
		}
		if bd.ID != 0 {
			b.ID = bd.ID
		}
		if !b.IsShell() && b.Type != world.BuildingParking {
			b.Rotation = bd.Rotation
		}
		// Parking-only state. The lot is its rectangle (X, Z centre,
		// Rotation, LotSize); stalls and MaxCars are re-derived. The
		// entrance node and driveway edge come back with the road graph
		// below; we just relink their IDs here.
		if b.Type == world.BuildingParking {
			b.DrivewayNodeIDs = bd.DrivewayNodeIDs
			b.DriveEdge = bd.DriveEdge
			w.SetLotRect(b, world.FootprintRect{Center: mgl32.Vec2{bd.X, bd.Z}, HalfX: bd.LotSize[0] / 2, HalfZ: bd.LotSize[1] / 2, Rotation: bd.Rotation})
		}
		b.SnowGunEnabled = bd.SnowGunEnabled
	}

	// Restore snowcats, snowmobiles, and patrollers. Service buildings
	// spawn their patrollers as their tiles load (and saved IDs replace
	// theirs only after), so start over: restore each saved vehicle or
	// patroller whose building still has a garage or patrol, then top up
	// or trim every building's patrol to its tiles and its vehicles to its
	// garage space.
	w.Snowcats, w.Snowmobiles, w.Patrollers = nil, nil, nil
	shedByID := make(map[uint64]*world.Building)
	hutByID := make(map[uint64]*world.Building)
	for _, b := range w.Buildings {
		if b.TileCount(world.ServiceGarage) > 0 {
			shedByID[b.ID] = b
		}
		if b.TileCount(world.ServicePatrol) > 0 {
			hutByID[b.ID] = b
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
		cat.Status = world.CatStatus(cd.Status)
	}
	for _, pd := range data.Patrollers {
		hut := hutByID[pd.HutID]
		if hut == nil {
			continue
		}
		p := w.SpawnPatroller(hut)
		if pd.ID != 0 {
			p.ID = pd.ID
		}
		p.Pos = mgl32.Vec3{pd.Pos[0], pd.Pos[1], pd.Pos[2]}
		p.Heading = pd.Heading
		p.State = world.PatrollerState(pd.State)
		p.SnowmobileID, p.TargetGuestID, p.ActionTimer = pd.Snowmobile, pd.Target, pd.Timer
		p.OnSkis, p.LiftID, p.LiftProgress = pd.OnSkis, pd.Lift, pd.LiftT
		p.TargetPos = mgl32.Vec3{pd.TargetPos[0], pd.TargetPos[1], pd.TargetPos[2]}
	}
	for _, md := range data.Snowmobiles {
		g := shedByID[md.GarageID]
		if g == nil {
			continue
		}
		m := w.AddSnowmobile(g)
		if md.ID != 0 {
			m.ID = md.ID
		}
		m.Pos, m.Heading = mgl32.Vec3{md.Pos[0], md.Pos[1], md.Pos[2]}, md.Heading
		m.InGarage, m.TakenBy = md.InGarage, md.TakenBy
	}
	w.SettleSnowmobiles()
	for _, b := range w.Buildings {
		if b.IsShell() {
			w.SyncFleet(b)
			w.TrimGarage(b)
		}
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
		// Restore helicopter phase and flight progress for HeliLift saves.
		if lift.IsHeli() && lift.HeliState != nil {
			lift.HeliState.Phase = world.HeliPhase(ld.HeliPhase)
			lift.HeliState.Progress = ld.HeliProgress
		}
		lift.Staff.LineAttendant = ld.LineAttendant
		// Restore queue lane config for LiftDouble lifts.
		lift.QueueConfig = world.LiftQueueConfig{
			LeftLines:   ld.LeftLines,
			RightLines:  ld.RightLines,
			SingleRider: ld.SingleRider,
		}
		if ld.LeftLines > 0 || ld.RightLines > 0 || ld.SingleRider {
			lift.RebuildLines()
		}
	}

	// Restore guests. Every row in data.Guests rehydrates into a *Guest in
	// w.Guests (the master catchment, including dormant entries); rows with
	// State==OnMountain also get a pointer into w.OnMountain so the sim
	// ticks them. IDs are preserved so chair / queue references resolve.
	legacySkill := isLegacySkillEnum(data.Guests)
	for _, gd := range data.Guests {
		if legacySkill {
			gd.Skill = world.SkillInTier(int(gd.Skill), rand.New(rand.NewSource(int64(gd.ID))))
		}
		var id uint64
		if gd.ID != 0 {
			id = gd.ID
		} else {
			id = w.NextID()
		}
		traits := ai.TraitsFor(gd.Skill)
		arrivalOffset := world.RollArrivalOffset(rand.New(rand.NewSource(int64(id) ^ 0x5eed)))
		if gd.ArrivalOffset != nil {
			arrivalOffset = *gd.ArrivalOffset
		}
		if len(gd.Tastes) == int(ai.TasteCount) {
			copy(traits.Tastes[:], gd.Tastes)
		} else {
			// Saved before tastes: roll them, the same for this guest on
			// every load, as legacy skills are above.
			traits.Tastes = world.RollTastes(gd.Skill, rand.New(rand.NewSource(int64(id))))
		}
		traits.DailyBudget = world.DailyBudgetFor(gd.Skill)
		g := &world.Guest{
			ID:               id,
			Name:             gd.Name,
			Discipline:       world.Discipline(gd.Discipline),
			Traits:           traits,
			VisitsPerSeason:  gd.VisitsPerSeason,
			ArrivalOffset:    arrivalOffset,
			Leaving:          leavingFromData(gd.Leaving),
			HomeEntryID:      gd.HomeEntry,
			GroupID:          gd.Group,
			VisitsThisSeason: gd.VisitsThisSeason,
			LifetimeVisits:   gd.LifetimeVisits,
			LastStars:        gd.LastStars,
			State:            world.GuestState(gd.State),
			CarID:            gd.CarID,
			CarLot:           gd.CarLot,
		}
		if gd.LastVisitUnix != 0 {
			g.LastVisit = time.Unix(gd.LastVisitUnix, 0).UTC()
		}
		g.SeasonPassExpiry = gd.SeasonPassExpiry
		g.HasSeasonPass = gd.HasSeasonPass
		if g.State == world.OnMountain {
			g.DayTicketDue = gd.DayTicketDue
			g.DayTicketPaid = gd.DayTicketPaid
			g.HasDayTicket = gd.HasDayTicket
			g.RemainingBudget = gd.RemainingBudget
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
			energy := gd.Energy
			if energy <= 0 {
				energy = 1.0
			}
			g.Energy = energy
			hunger := gd.Hunger
			if hunger <= 0 {
				hunger = 1.0
			}
			g.Hunger = hunger
			thirst := gd.Thirst
			if thirst <= 0 {
				thirst = 1.0
			}
			g.Thirst = thirst
			g.SkisOn, g.NeedsGear = gd.SkisOn, gd.NeedsGear
			if sk := gd.Skis; sk != nil {
				g.Stash = world.SkiStash{Out: true, RackID: sk.Rack, Slot: sk.Slot, Pos: mgl32.Vec2{sk.Pos[0], sk.Pos[1]}, Yaw: sk.Yaw, Lying: sk.Lying}
			}
			for _, m := range gd.Moments {
				g.Moments = append(g.Moments, world.Moment{Kind: ai.ThoughtKind(m.Kind), N: m.N, Context: m.Context})
			}
			g.QualitySum, g.QualityUses = gd.QualitySum, gd.QualityUses
			g.TrailTally, g.LiftTally = tallyFromData(gd.TrailTally), tallyFromData(gd.LiftTally)
			g.Balance = 1.0
			if len(gd.PlanSteps) > 0 {
				steps := make([]ai.PlanAction, len(gd.PlanSteps))
				for si, pd := range gd.PlanSteps {
					steps[si] = ai.PlanAction{
						Kind:    ai.PlanActionKind(pd.Kind),
						LiftID:  pd.LiftID,
						BldgID:  pd.BldgID,
						TrailID: pd.TrailID,
						Via:     pd.Via,
						Use:     ai.Offer(pd.Use),
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

	// Doors face the nearest lift or parking lot, which only exist now.
	w.RefreshAllDoors()

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
		// Per-lane queues: restore guest pointers into each line in order.
		for li, ids := range ld.LineQueueIDs {
			if li >= len(lift.Lines) {
				break
			}
			for _, gid := range ids {
				if a := guestByID[gid]; a != nil {
					lift.Lines[li].Guests = append(lift.Lines[li].Guests, a)
				}
			}
		}
	}

	// Restore road graph. Nodes first so edges can reference them by ID.
	// Old saves without road data leave both slices empty — there's no
	// implicit road network to fall back to.
	for _, nd := range data.RoadNodes {
		pos := mgl32.Vec2{nd.X, nd.Z}
		n := w.AddRoadNode(pos, world.RoadNodeKind(nd.Kind))
		n.Name, n.Pool = nd.Name, nd.Pool
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

	// Restore parcels and derive the terrain accessibility grid.
	w.SetSkiArea(skiAreaFromData(data.SkiArea))
	for _, pd := range data.Parcels {
		w.Parcels = append(w.Parcels, world.Parcel{
			ID:    pd.ID,
			Name:  pd.Name,
			State: world.ParcelState(pd.State),
			Price: pd.Price,
			Cells: pd.Cells,
		})
	}
	w.ApplyParcels()

	// Restore trails. TrailGraph is derived on load rather than persisted.
	for _, td := range data.Trails {
		if len(td.Nodes) < 2 && len(td.Outline) < 3 {
			continue // painted before trails were drawn: dropped
		}
		t := w.PlaceTrail(td.Name, world.TerrainDifficulty(td.Difficulty))
		if td.ID != 0 {
			t.ID = td.ID
		}
		t.Kind = world.TrailKind(td.Kind)
		t.Groomed = td.Groomed && !t.Kind.IsArea()
		for _, n := range td.Nodes {
			t.Nodes = append(t.Nodes, world.TrailNode{Pos: mgl32.Vec2{n[0], n[1]}, Width: n[2]})
		}
		for _, p := range td.Outline {
			t.Outline = append(t.Outline, mgl32.Vec2{p[0], p[1]})
		}
		t.Start, t.End = trailEndFromData(td.Start), trailEndFromData(td.End)
	}
	if len(w.Trails) > 0 {
		w.RebuildTrailGraph()
	}
	for _, fd := range data.Footpaths {
		f := &world.Footpath{ID: fd.ID}
		for _, n := range fd.Nodes {
			f.Nodes = append(f.Nodes, world.TrailNode{Pos: mgl32.Vec2{n[0], n[1]}, Width: n[2]})
		}
		if len(f.Nodes) >= 2 {
			w.SetMinNextID(f.ID)
			w.Footpaths = append(w.Footpaths, f)
		}
	}
	w.RebuildFootpaths()
	for _, rd := range data.Ropes {
		r := &world.Rope{ID: rd.ID}
		for _, n := range rd.Nodes {
			r.Nodes = append(r.Nodes, mgl32.Vec2{n[0], n[1]})
		}
		if len(r.Nodes) >= 2 {
			w.SetMinNextID(r.ID)
			w.Ropes = append(w.Ropes, r)
		}
	}
	w.RebuildRopes()

	// Validate parking driveways now that road nodes are in place.
	// EnsureParkingDriveway is idempotent — a parking lot whose
	// DrivewayNodeID resolves to an existing node is left alone; one
	// with a missing or zero ID gets a fresh driveway. Covers older
	// saves that predate the driveway field and any corrupted graphs.
	for _, b := range w.Buildings {
		if b.Type == world.BuildingParking {
			w.RefreshParkingLot(b, false)
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
	for _, p := range w.Patrollers {
		if p.ID > maxID {
			maxID = p.ID
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
	loadCars(w, data.Cars)
	for _, c := range w.Cars {
		maxID = max(maxID, c.ID)
	}
	w.SetMinNextID(maxID)

	// Make the guest pool match the road entries' pools (each guest lives
	// beyond one entry), or seed the default pool for a map without
	// entries, so pools edited in the editor take effect on the next load.
	if m := data.GroupMix; len(m) == 3 {
		w.GroupMix = world.GroupMix{MeanSize: m[0], Lessons: m[1], Mixed: m[2]}
	}
	guestSeed := w.Seed
	if guestSeed == 0 {
		guestSeed = 1 // legacy saves with no seed: stable fallback
	}
	world.SyncGuestPool(w, guestSeed)

	// Rehydrate the history ring. Absent in the save → allocate an
	// empty *History so the sim starts recording immediately.
	w.History = historyFromData(data.History)
	eventsFromData(&w.Events, data.Events)
	w.SimTime = data.SimTime
	if d, err := time.Parse(startDateLayout, data.StartDate); err == nil {
		w.StartDate = d
	}
	w.Scenario = scenarioInfoOf(data)
	if len(data.Geo) == 4 {
		w.Geo = &world.GeoBounds{MinLat: data.Geo[0], MaxLat: data.Geo[1], MinLon: data.Geo[2], MaxLon: data.Geo[3]}
	}
	w.BaseAltitude = data.BaseAltitude
	w.TimeZone = data.TimeZone
	w.Climate = climateFromData(data.Climate)
	w.TerrainBase = terrainBaseFromData(data.TerrainBase)
	if data.OpenHour > 0 || data.CloseHour > 0 {
		w.OpenHour, w.CloseHour = data.OpenHour, data.CloseHour
	}
	daySec := data.DaySec
	if daySec == 0 {
		daySec = world.LegacySecondsPerSimDay
	}
	if daySec != world.SecondsPerSimDay {
		rescaleClocks(w, world.SecondsPerSimDay/daySec)
	}

	return w
}

// rescaleClocks multiplies every absolute sim-clock value in w by k, for
// saves written with a different day length. Durations (timers, lift
// progress) are in movement seconds and stay as they are.
func rescaleClocks(w *world.World, k float64) {
	w.SimTime *= k
	w.Events.ScaleTimes(k)
	for _, g := range w.Guests {
		g.SeasonPassExpiry *= k
	}
}

// loadServiceBuilding restores a service building: its grid (Origin,
// Rotation) and its tiles.
func loadServiceBuilding(w *world.World, bd BuildingData) *world.Building {
	tiles := make(map[[2]int]world.Service, len(bd.Tiles))
	for _, t := range bd.Tiles {
		tiles[[2]int{t[0], t[1]}] = world.Service(t[2])
	}
	b := w.PlaceServiceBuilding(mgl32.Vec2{bd.OriginX, bd.OriginZ}, bd.Rotation, tiles, bd.StyleSeed)
	b.Kind, b.Storeys = world.ShellKind(bd.Kind), bd.Storeys
	if bd.FloorSet {
		b.FloorY, b.FloorSet = bd.FloorY, true
	}
	if bd.MealPrice > 0 {
		b.MealPrice = bd.MealPrice
	}
	b.FreeWater = bd.FreeWater
	if bd.DrinkPrice > 0 {
		b.DrinkPrice = bd.DrinkPrice
	}
	if bd.Rental > 0 {
		b.RentalPrice = bd.Rental
	}
	if bd.Quality != nil {
		b.Quality = *bd.Quality
	}
	return b
}

// saveTiles lists a service building's cells with their services.
func saveTiles(b *world.Building) [][3]int {
	out := make([][3]int, len(b.Cells))
	for i, c := range b.Cells {
		out[i] = [3]int{c[0], c[1], int(b.ServiceAt(c))}
	}
	return out
}

// mogulPixels is the mogul map to save, in line with every cell's
// MogulSize, or nil when there are no moguls.
func mogulPixels(t *world.Terrain) []byte {
	t.SyncMoguls()
	return t.Moguls.Bytes()
}

// groomPixels is the groom map to save, or nil when nothing is groomed.
func groomPixels(t *world.Terrain) []byte {
	if t.Groom.Empty() {
		return nil
	}
	return t.Groom.Bytes()
}

func geoToData(g *world.GeoBounds) []float64 {
	if g == nil {
		return nil
	}
	return []float64{g.MinLat, g.MaxLat, g.MinLon, g.MaxLon}
}

func climateToData(c *world.Climate) *ClimateData {
	if c == nil {
		return nil
	}
	d := &ClimateData{Source: c.Source, RefAltitude: c.RefAltitude, WindDeg: c.WindDeg, Months: make([]ClimateMonthData, 12)}
	for i, m := range c.Months {
		d.Months[i] = ClimateMonthData(m)
	}
	return d
}

// climateFromData is nil for saves without a climate or with a broken one.
func climateFromData(d *ClimateData) *world.Climate {
	if d == nil || len(d.Months) != 12 {
		return nil
	}
	c := &world.Climate{Source: d.Source, RefAltitude: d.RefAltitude, WindDeg: d.WindDeg}
	for i, m := range d.Months {
		c.Months[i] = world.ClimateMonth(m)
	}
	return c
}

func terrainBaseToData(b *world.TerrainBase) *TerrainBaseData {
	if b == nil {
		return nil
	}
	d := &TerrainBaseData{
		Geo:           []float64{b.Geo.MinLat, b.Geo.MaxLat, b.Geo.MinLon, b.Geo.MaxLon},
		W:             b.W,
		H:             b.H,
		Detail:        b.Detail,
		Heights:       b.HeightsBytes(),
		RoadNote:      b.RoadNote,
		LidarCoverage: b.LidarCoverage,
		LidarNote:     b.LidarNote,
		LayersOff:     b.LayersOff,
		Strengths:     b.Strengths,
	}
	for _, r := range b.Roads {
		d.Roads = append(d.Roads, BaseRoadData{Name: r.Name, Kind: r.Kind, Width: r.Width, Tunnel: r.Tunnel, Path: flatPath(r.Path)})
	}
	for _, l := range b.Lifts {
		d.Lifts = append(d.Lifts, BaseLiftData{Name: l.Name, Kind: l.Kind, Seats: l.Seats, Path: flatPath(l.Path)})
	}
	for _, r := range b.Runs {
		d.Runs = append(d.Runs, BaseRunData{Name: r.Name, Difficulty: r.Difficulty, Area: r.Area, Path: flatPath(r.Path)})
	}
	for _, s := range b.Streams {
		d.Streams = append(d.Streams, BaseStreamData{Name: s.Name, Kind: s.Kind, Intermittent: s.Intermittent, Path: flatPath(s.Path)})
	}
	for _, l := range b.Lakes {
		ld := BaseLakeData{Name: l.Name, Kind: l.Kind}
		for _, p := range l.Paths {
			ld.Paths = append(ld.Paths, flatPath(p))
		}
		d.Lakes = append(d.Lakes, ld)
	}
	for _, a := range b.Areas {
		ad := BaseAreaData{Name: a.Name}
		for _, p := range a.Paths {
			ad.Paths = append(ad.Paths, flatPath(p))
		}
		d.Areas = append(d.Areas, ad)
	}
	return d
}

// flatPath is a (lat, lon) path as flat lat, lon pairs.
func flatPath(ps [][2]float64) []float64 {
	out := make([]float64, 0, 2*len(ps))
	for _, p := range ps {
		out = append(out, p[0], p[1])
	}
	return out
}

// pairPath reads a path written by flatPath.
func pairPath(fs []float64) [][2]float64 {
	var out [][2]float64
	for k := 0; k+1 < len(fs); k += 2 {
		out = append(out, [2]float64{fs[k], fs[k+1]})
	}
	return out
}

// terrainBaseFromData is nil for a missing or unreadable base, which
// only costs the editor its Layers panel.
func terrainBaseFromData(d *TerrainBaseData) *world.TerrainBase {
	if d == nil || len(d.Geo) != 4 || d.W < 2 || d.H < 2 {
		return nil
	}
	b := &world.TerrainBase{
		Geo:           world.GeoBounds{MinLat: d.Geo[0], MaxLat: d.Geo[1], MinLon: d.Geo[2], MaxLon: d.Geo[3]},
		W:             d.W,
		H:             d.H,
		Detail:        d.Detail,
		RoadNote:      d.RoadNote,
		LidarCoverage: d.LidarCoverage,
		LidarNote:     d.LidarNote,
		LayersOff:     d.LayersOff,
		Strengths:     d.Strengths,
	}
	if err := b.SetHeightsBytes(d.Heights); err != nil {
		fmt.Println("save:", err)
		return nil
	}
	for _, r := range d.Roads {
		b.Roads = append(b.Roads, world.BaseRoad{Name: r.Name, Kind: r.Kind, Width: r.Width, Tunnel: r.Tunnel, Path: pairPath(r.Path)})
	}
	for _, l := range d.Lifts {
		b.Lifts = append(b.Lifts, world.BaseLift{Name: l.Name, Kind: l.Kind, Seats: l.Seats, Path: pairPath(l.Path)})
	}
	for _, r := range d.Runs {
		b.Runs = append(b.Runs, world.BaseRun{Name: r.Name, Difficulty: r.Difficulty, Area: r.Area, Path: pairPath(r.Path)})
	}
	for _, s := range d.Streams {
		b.Streams = append(b.Streams, world.BaseStream{Name: s.Name, Kind: s.Kind, Intermittent: s.Intermittent, Path: pairPath(s.Path)})
	}
	for _, l := range d.Lakes {
		bl := world.BaseLake{Name: l.Name, Kind: l.Kind}
		for _, p := range l.Paths {
			bl.Paths = append(bl.Paths, pairPath(p))
		}
		b.Lakes = append(b.Lakes, bl)
	}
	for _, a := range d.Areas {
		ba := world.BaseArea{Name: a.Name}
		for _, p := range a.Paths {
			ba.Paths = append(ba.Paths, pairPath(p))
		}
		b.Areas = append(b.Areas, ba)
	}
	return b
}

// lakeDepthBytes is lake depths for saving, in whole decimetres up to
// 25.5 m.
func lakeDepthBytes(d []float32) []byte {
	if d == nil {
		return nil
	}
	out := make([]byte, len(d))
	for k, v := range d {
		out[k] = byte(min(max(math.Round(float64(v)*10), 0), 255))
	}
	return out
}

func lakesToData(ls []world.Lake) []LakeData {
	var out []LakeData
	for _, l := range ls {
		out = append(out, LakeData{Name: l.Name, Altitude: l.Altitude, AreaHa: l.AreaHa, MaxDepth: l.MaxDepth, Frost: l.Frost, Thaw: l.Thaw})
	}
	return out
}

func goalsToData(gs []world.Goal) []GoalData {
	var out []GoalData
	for _, g := range gs {
		out = append(out, GoalData{Kind: uint8(g.Kind), Target: g.Target, Days: g.Days, Season: g.Season, Bonus: g.Bonus})
	}
	return out
}

func progressToData(ps []world.GoalProgress) []GoalProgressData {
	var out []GoalProgressData
	for _, p := range ps {
		out = append(out, GoalProgressData{Met: p.Met, MetDay: p.MetDay, Streak: p.Streak, Best: p.Best, Failed: p.Failed})
	}
	return out
}

// materialBytes is the material map to save, or nil when there is none.
func materialBytes(t *world.Terrain) []byte {
	if t.Material == nil {
		return nil
	}
	return t.Material.Bytes()
}

// detailBytes is the terrain detail to save, or nil when there is none.
func detailBytes(t *world.Terrain) []byte {
	if t.Detail == nil {
		return nil
	}
	return t.Detail.Bytes()
}

// loadCars restores the saved cars and links their guests. A guest whose
// car didn't survive goes home (in a car) or leaves from any lot (on the
// mountain).
func loadCars(w *world.World, data []CarData) {
	byID := make(map[uint64]*world.Guest, len(w.Guests))
	for _, g := range w.Guests {
		byID[g.ID] = g
	}
	carIDs := map[uint64]bool{}
	for _, cd := range data {
		c := &world.Car{
			ID: cd.ID, Kind: world.CarKind(cd.Kind), Roof: world.CarRoof(cd.Roof),
			Entry: cd.Entry, Lot: cd.Lot, Stall: cd.Stall,
			State: world.CarState(cd.State), Route: cd.Route, Leg: cd.Leg, D: cd.D,
			Speed: cd.Speed, Pos: mgl32.Vec2{cd.Pos[0], cd.Pos[1]},
			Heading: cd.Heading, InLot: cd.InLot,
		}
		for _, id := range cd.Guests {
			if g := byID[id]; g != nil && g.CarID == c.ID {
				c.Guests = append(c.Guests, g)
			}
		}
		w.Cars = append(w.Cars, c)
		carIDs[c.ID] = true
	}
	for _, g := range w.Guests {
		if g.CarID != 0 && !carIDs[g.CarID] {
			g.CarID, g.CarLot = 0, 0
			if g.State == world.InCar {
				g.State = world.AtHome
			}
		}
	}
}

// leavingToData saves a pending departure, nil when there is none.
func leavingToData(l world.Leaving) *LeavingData {
	if !l.Pending {
		return nil
	}
	r := l.Review
	return &LeavingData{Review: ReviewData{
		Level: r.Level, Stars: r.Stars, Quality: r.Quality, Kind: uint8(r.Kind), Count: r.Count,
		Context: r.Context, Annoyances: r.Annoyances, NoRuns: r.NoRuns,
	}, Reason: uint8(l.Reason)}
}

// tallyToData and tallyFromData save and restore a guest's run tally.
func tallyToData(t []world.RunTally) []RunTallyData {
	var d []RunTallyData
	for _, r := range t {
		d = append(d, RunTallyData{ID: r.ID, Runs: r.Runs, Great: r.Great})
	}
	return d
}

func tallyFromData(d []RunTallyData) []world.RunTally {
	var t []world.RunTally
	for _, r := range d {
		t = append(t, world.RunTally{ID: r.ID, Runs: r.Runs, Great: r.Great})
	}
	return t
}

// leavingFromData restores a pending departure.
func leavingFromData(d *LeavingData) world.Leaving {
	if d == nil {
		return world.Leaving{}
	}
	r := d.Review
	return world.Leaving{Pending: true, Review: world.Review{
		Level: r.Level, Stars: r.Stars, Quality: r.Quality, Kind: ai.ThoughtKind(r.Kind), Count: r.Count,
		Context: r.Context, Annoyances: r.Annoyances, NoRuns: r.NoRuns,
	}, Reason: ai.DepartReason(d.Reason)}
}

// skiAreaData is the ski-area boundary for saving.
func skiAreaData(w *world.World) [][][2]float32 {
	out := make([][][2]float32, 0, len(w.SkiArea))
	for _, o := range w.SkiArea {
		pts := make([][2]float32, len(o.Points))
		for i, p := range o.Points {
			pts[i] = [2]float32{p[0], p[1]}
		}
		out = append(out, pts)
	}
	return out
}

// skiAreaFromData is a saved ski-area boundary.
func skiAreaFromData(d [][][2]float32) []world.SkiAreaOutline {
	out := make([]world.SkiAreaOutline, 0, len(d))
	for _, pts := range d {
		o := world.SkiAreaOutline{Points: make([]mgl32.Vec2, len(pts))}
		for i, p := range pts {
			o.Points[i] = mgl32.Vec2{p[0], p[1]}
		}
		out = append(out, o)
	}
	return out
}

func trailEndToData(e world.TrailEnd) *TrailEndData {
	if !e.Set {
		return nil
	}
	return &TrailEndData{Kind: uint8(e.Kind), ID: e.ID}
}

func trailEndFromData(d *TrailEndData) world.TrailEnd {
	if d == nil {
		return world.TrailEnd{}
	}
	return world.TrailEnd{Set: true, Kind: world.EdgeKind(d.Kind), ID: d.ID}
}

// groupMixData is m as saved: nil for the default.
func groupMixData(m world.GroupMix) []float32 {
	if m == (world.GroupMix{}) {
		return nil
	}
	return []float32{m.MeanSize, m.Lessons, m.Mixed}
}
