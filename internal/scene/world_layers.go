package scene

import (
	"math"
	"time"

	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/world"
)

// worldLayer is a Terrain layer that dresses the ground without moving
// it, so switching it keeps everything built. They run after the ground
// layers (geo.Layers), in order, and are listed after them in the panel.
// Later ones can read what earlier ones made (trees keep off the
// material map's rock), so rerunning one reruns those after it.
type worldLayer struct {
	ID, Name string
	// fixed layers have no strength slider.
	fixed bool
	run   func(w *world.World, c *layerCache)
	clear func(w *world.World)
	// note is a word on how the layer ran here, or "".
	note func(w *world.World) string
}

// layerCache keeps what the world layers derive from the ground between
// runs. Clear it when the ground changes.
type layerCache struct {
	fields *elevFields
	pack   *sim.SeasonSnowpack
}

func (c *layerCache) fieldsFor(t *world.Terrain) *elevFields {
	if c.fields == nil {
		c.fields = computeElevFields(t)
	}
	return c.fields
}

var worldLayers = []worldLayer{
	{
		ID: "material", Name: "Auto material", fixed: true,
		run:   runMaterialLayer,
		clear: clearMaterial,
		note: func(w *world.World) string {
			if w.Terrain.Detail == nil {
				return "needs lidar"
			}
			return ""
		},
	},
	{
		ID: "trees", Name: "Auto trees",
		run: func(w *world.World, c *layerCache) {
			f := c.fieldsFor(w.Terrain)
			// Strength sets the coverage, 55% at the default and up to 95%.
			coverage := 0.55 * world.LayerScale(w.TerrainBase.Strength("trees"), 0, 0.95/0.55)
			f.generateTreeCover(w.Terrain, 24, float32(coverage), treelineFrac(w, f), layerSeed(w))
			removeTreesOnBare(w.Terrain)
		},
		clear: func(w *world.World) { w.Terrain.ClearAllTrees() },
		note: func(w *world.World) string {
			if w.Climate == nil {
				return "no climate"
			}
			return ""
		},
	},
	{
		ID: "snow", Name: "Auto snow",
		run:   runSnowLayer,
		clear: clearSnow,
		note: func(w *world.World) string {
			if w.Climate == nil {
				return "no climate"
			}
			return ""
		},
	},
}

// WorldLayerIDs lists the world layers' IDs, in order.
func WorldLayerIDs() []string {
	var ids []string
	for _, l := range worldLayers {
		ids = append(ids, l.ID)
	}
	return ids
}

// DressWorld runs the world layers on a newly imported world, first
// moving its start date to the season's opening day when Auto snow is
// on and there's a climate. progress, if set, hears each layer's name.
func DressWorld(w *world.World, progress func(name string)) map[string]time.Duration {
	return dressWorld(w, &layerCache{}, progress)
}

// RedressWorld reruns the world layers that are on, keeping the start
// date, for a scenario saved before a layer existed or changed.
func RedressWorld(w *world.World, progress func(name string)) map[string]time.Duration {
	return runWorldLayers(w, &layerCache{}, progress)
}

func dressWorld(w *world.World, c *layerCache, progress func(name string)) map[string]time.Duration {
	if w.Climate != nil && w.TerrainBase.LayerOn("snow") {
		w.StartDate = snowOpeningDay(w, c)
	}
	return runWorldLayers(w, c, progress)
}

// runWorldLayers runs every world layer that's on, leaving what the
// others left alone, then clears around what's built. progress, if set,
// hears each layer's name before it runs. It returns how long each took,
// by ID.
func runWorldLayers(w *world.World, c *layerCache, progress func(name string)) map[string]time.Duration {
	took := map[string]time.Duration{}
	for _, l := range worldLayers {
		if !w.TerrainBase.LayerOn(l.ID) {
			continue
		}
		if progress != nil {
			progress(l.Name)
		}
		start := time.Now()
		l.run(w, c)
		took[l.ID] = time.Since(start)
	}
	restampClearings(w)
	return took
}

// restampClearings clears the trees and snow that the generators put
// back under buildings, parking, lifts and roads.
func restampClearings(w *world.World) {
	for _, b := range w.Buildings {
		if b.IsPainted() {
			continue
		}
		halfX, halfZ := buildingFootprint(b.Type)
		clearBuildingTrees(w.Terrain, b.Pos, halfX, halfZ, b.Rotation)
	}
	replowParkingLots(w)
	for _, lift := range w.Lifts {
		clearLiftCorridor(w.Terrain, lift.Base, lift.Top, liftCorridorHalfWidth)
	}
	applyRoadCellState(w)
}

// layerSeed is the world layers' noise seed: fixed for a place, so a
// layer switched off and on again comes back the same.
func layerSeed(w *world.World) int64 {
	if w.Geo == nil {
		return 1
	}
	return int64(math.Float64bits(w.Geo.MinLat) ^ math.Float64bits(w.Geo.MinLon))
}

// treelineFrac is the climate's treeline as a fraction of the map's
// elevation range (above 1 when it's over the top), or 0.7 with no
// climate.
func treelineFrac(w *world.World, f *elevFields) float32 {
	if w.Climate == nil || f.maxE <= f.minE {
		return 0.7
	}
	return (w.Climate.TreelineAltitude() - w.BaseAltitude - f.minE) / (f.maxE - f.minE)
}

// seasonYear is the year of the September that starts date's season.
func seasonYear(date time.Time) int {
	if date.Month() < time.September {
		return date.Year() - 1
	}
	return date.Year()
}

// snowpackFor is the season snowpack for w's start date, from the cache
// when it's for the same season.
func snowpackFor(w *world.World, c *layerCache) *sim.SeasonSnowpack {
	year := seasonYear(w.StartDate)
	if c.pack == nil || c.pack.Start.Year() != year {
		f := c.fieldsFor(w.Terrain)
		const margin = 100 // metres, for later raising and lowering
		c.pack = sim.NewSeasonSnowpack(w.Climate, sim.SiteOf(w).LatDeg, year,
			w.BaseAltitude+f.minE-margin, w.BaseAltitude+f.maxE+margin)
	}
	return c.pack
}

// Opening snow: SWE on flat open ground at a quarter of the way up the
// map that makes a resort open for the season, about half a metre of
// settled snow.
const (
	openingSWE      = 0.15
	openingAltFrac  = 0.25
	openingFallback = 15 // December, when the snow never gets there
)

// snowDepthScale is how much Auto snow's strength multiplies the
// season's snowpack: none at 0, a typical season at the default, and
// two and a half times it at full strength, a big year.
func snowDepthScale(w *world.World) float32 {
	return float32(world.LayerScale(w.TerrainBase.Strength("snow"), 0, 2.5))
}

// snowOpeningDay is the day w's snowpack, at Auto snow's strength, is
// deep enough to open on, or mid-December when it never is.
func snowOpeningDay(w *world.World, c *layerCache) time.Time {
	pack := snowpackFor(w, c)
	f := c.fieldsFor(w.Terrain)
	alt := w.BaseAltitude + f.minE + openingAltFrac*(f.maxE-f.minE)
	if k := snowDepthScale(w); k > 0.01 {
		if d, ok := pack.OpeningDay(alt, openingSWE/k); ok {
			return d
		}
	}
	return time.Date(pack.Start.Year(), time.December, openingFallback, 0, 0, 0, 0, time.UTC)
}

// runSnowLayer lays the snow a typical season leaves by the start date,
// shaped by the ground and scaled by the layer's strength. Without a
// climate it falls back to the Auto tool's generator.
func runSnowLayer(w *world.World, c *layerCache) {
	t := w.Terrain
	f := c.fieldsFor(t)
	t.Groom.Clear()
	k := snowDepthScale(w)
	if w.Climate == nil {
		f.generateSnowCover(t, 2*k, 0.3, 0.7, 270, layerSeed(w))
		return
	}
	pack := snowpackFor(w, c)
	day := pack.Day(w.StartDate)
	shade := sim.SnowShade(t, sim.SiteOf(w).LatDeg, w.StartDate)
	drift := f.newSnowDrift(t, treelineFrac(w, f), w.Climate.WindDeg, layerSeed(w))
	const maxDrift = 2.5
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			cell := &t.Cells[x][z]
			gx, gz := t.GradientAt(x, z)
			swe, recent := pack.At(day, w.BaseAltitude+cell.GroundElevation, gx, gz, shade[x*t.Height+z])
			d := min(drift.at(x, z, f.elevFrac(cell.GroundElevation)), maxDrift) * k
			cell.Base = (swe - recent) * d
			cell.Top = world.SnowLayer{}
			if recent > 0 {
				cell.Top = world.SnowLayer{Accumulation: recent * d, Kind: world.KindPowder}
			}
		}
	}
	t.SnowDirty = true
}

func clearSnow(w *world.World) {
	t := w.Terrain
	for x := range t.Cells {
		for z := range t.Cells[x] {
			t.Cells[x][z].Base = 0
			t.Cells[x][z].Top = world.SnowLayer{}
		}
	}
	t.Groom.Clear()
	t.SnowDirty = true
}
