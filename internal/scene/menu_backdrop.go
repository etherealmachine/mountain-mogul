package scene

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/world"
)

// The menus' backdrop, the Factorio way: real gameplay. A built-out
// resort (assets/menu.save, baked by tools/menuscene) runs live
// behind the menus at the normal pace, filmed through a perspective
// camera that cuts between slow shots: orbiting a lift's base and its
// line, dollying up beside a lift. Until it has loaded, and if it can't,
// the menus sit on a plain winter sky.
//
// The renderer holds one world at a time, so the backdrop is a single
// package-level instance shared by every menu screen (start menu,
// scenario picker, save list, testbeds): they tick it in Update and draw
// it in Render, before their UI. A game or the editor starting drops it
// (releaseMenuBackdrop), and the menu loads it afresh when it's next on
// screen. Loading the save runs off the main thread; uploading it to the
// GPU runs on it, on the first menu frame after.

const (
	menuSceneFile = "menu.save" // in the assets folder, not scenarios/, so it isn't offered as one
	menuShotLen   = 18.0        // seconds per shot
	// menuPace is the backdrop's speed against the game's normal pace
	// (about 4× real time): half, so chairs and skiers move about
	// twice real speed, calm enough to watch.
	menuPace     = 0.5
	menuShotFade = 0.7 // seconds fading through the sky at each cut
	// The clock winds back to menuDayFrom when it reaches menuDayTo, so
	// the menu never sees the lifts close.
	menuDayFrom, menuDayTo = 10.5, 15.0
)

var (
	menuSkyTop = mgl32.Vec4{0.16, 0.27, 0.52, 1} // introSky
	menuSkyLow = mgl32.Vec4{0.74, 0.82, 0.95, 1}
)

type menuBackdrop struct {
	world *world.World
	sim   *sim.Simulation
	lifts []*world.Lift // sorted by ID, so the shots run in a fixed order
	time  float32
	shot  int
	shotT float32
}

var (
	menuBD       *menuBackdrop
	menuBDLoad   chan *world.World // the load in flight, if any
	menuBDFailed bool              // the save didn't load; don't retry
)

// startMenuBackdropLoad starts loading the backdrop's save in the
// background, unless it's loaded or loading. The intro calls it so the
// world is ready by the time the start menu shows.
func startMenuBackdropLoad(assetDir string) {
	if menuBD != nil || menuBDLoad != nil || menuBDFailed {
		return
	}
	ch := make(chan *world.World, 1)
	menuBDLoad = ch
	go func() {
		w, _, err := save.LoadScenario(filepath.Join(assetDir, menuSceneFile))
		if err != nil {
			fmt.Printf("menu backdrop: %v\n", err)
			ch <- nil
			return
		}
		levelStationAprons(w)
		ch <- w
	}()
}

// releaseMenuBackdrop drops the backdrop, for a scene about to install
// its own world in the renderer (which resets the renderer's scene state
// itself). The camera goes back to the game's ortho view.
func releaseMenuBackdrop(r *render.Renderer) {
	if menuBD == nil && menuBDLoad == nil {
		return
	}
	menuBD, menuBDLoad = nil, nil
	r.Camera.Perspective = false
	r.Camera.Recalculate()
}

// tickMenuBackdrop advances the backdrop a frame, installing it once its
// save has loaded. Every menu screen calls it from Update.
func tickMenuBackdrop(app *engine.App, dt float64) {
	startMenuBackdropLoad(app.AssetDir)
	if menuBD == nil {
		select {
		case w := <-menuBDLoad:
			menuBDLoad = nil
			if w == nil {
				menuBDFailed = true
				return
			}
			menuBD = installMenuBackdrop(app.Renderer, w)
		default:
			return
		}
	}
	menuBD.update(app.Renderer, dt)
}

func installMenuBackdrop(r *render.Renderer, w *world.World) *menuBackdrop {
	r.ResetSceneState()
	w.TerrainBase = nil
	b := &menuBackdrop{world: w, sim: sim.NewSimulationWithSeed(w, 1)}
	b.sim.OnDayRollover = applyRoadCellState
	b.sim.SetResortOpen(true)
	// Always a bluebird day.
	today := b.sim.Weather.Today()
	today.State, today.CloudCover = sim.WeatherClear, 0.05
	b.sim.Weather.SetToday(today)

	r.BuildTerrainMesh(w.Terrain)
	w.Terrain.RestampTreeWells()
	r.BuildSnowSurfaceTex(w.Terrain)
	r.BuildGroomTex(w.Terrain)
	r.BuildMogulTex(w.Terrain)
	r.RebuildStaticBatch(w)
	r.RebuildRoads(w)
	for _, l := range w.Lifts {
		r.AddLiftCable(l, w.Terrain)
	}
	b.lifts = append(b.lifts, w.Lifts...)
	sort.Slice(b.lifts, func(i, j int) bool { return b.lifts[i].ID < b.lifts[j].ID })
	return b
}

func (b *menuBackdrop) update(r *render.Renderer, dt float64) {
	b.time += float32(dt)
	b.sim.TimeScale = paceScale(dayPaces[0]) * menuPace
	b.sim.Tick(dt)
	day := int(b.sim.SimTime / world.SecondsPerSimDay)
	if hour := (b.sim.SimTime - float64(day)*world.SecondsPerSimDay) / world.SimSecondsPerHour; hour >= menuDayTo {
		b.sim.SimTime = sim.SimTimeAt(day, menuDayFrom)
		b.world.SimTime = b.sim.SimTime
		b.sim.SkipClockEffects()
	}

	// Upload what the sim changed (as Scenario.Update does).
	t := b.world.Terrain
	r.FlushSnowDirty(t)
	r.FlushTerrainMaterial(t)
	if sd := t.Surface; sd != nil && sd.Dirty {
		r.FlushSnowSurface(t)
	}
	if g := t.Groom; g != nil && g.Dirty {
		r.FlushGroom(t)
	}
	r.FlushMoguls(t)

	b.shotT += float32(dt)
	if b.shotT >= menuShotLen {
		b.shot++
		b.shotT = 0
	}
}

// aim points the camera for the current shot. Shots go two to a lift, in
// turn: a slow orbit round its base, then a dolly up beside its line.
func (b *menuBackdrop) aim(c *render.Camera) {
	c.Perspective = true
	c.FOVDeg = 50
	if len(b.lifts) == 0 {
		c.EyePos, c.LookAt = mgl32.Vec3{0, 200, 0}, mgl32.Vec3{100, 0, 100}
		c.Recalculate()
		return
	}
	l := b.lifts[(b.shot/2)%len(b.lifts)]
	side := float32(1 - 2*((b.shot/2)%2)) // alternate sides lift to lift
	dir := l.Top.Sub(l.Base)
	length := dir.Len()
	dir = dir.Normalize()
	perp := mgl32.Vec2{-dir[1], dir[0]}.Mul(side)
	ground := b.world.Terrain.InterpolatedSurfaceElevationAt
	at := func(p mgl32.Vec2, up float32) mgl32.Vec3 { return mgl32.Vec3{p[0], ground(p[0], p[1]) + up, p[1]} }

	if b.shot%2 == 0 {
		// Orbit the base station, looking past it up the line.
		a := float64(b.shotT) * 0.014
		cs, sn := float32(math.Cos(a)), float32(math.Sin(a))
		// Start behind and to the side of the base, turn slowly.
		off := dir.Mul(-60).Add(perp.Mul(75))
		off = mgl32.Vec2{off[0]*cs - off[1]*sn, off[0]*sn + off[1]*cs}
		eye := l.Base.Add(off)
		look := l.Base.Add(dir.Mul(min(70, length*0.2)))
		c.EyePos = at(eye, 26)
		c.LookAt = at(look, 4)
	} else {
		// Dolly up beside the line, a little behind the chairs' climb.
		d := min(30+5*b.shotT, length*0.7)
		p := l.Base.Add(dir.Mul(d))
		eye := p.Sub(dir.Mul(35)).Add(perp.Mul(38))
		look := p.Add(dir.Mul(55))
		c.EyePos = at(eye, 20)
		c.LookAt = at(look, 9)
	}
	// Never inside a hill.
	c.EyePos[1] = max(c.EyePos[1], ground(c.EyePos[0], c.EyePos[2])+8)
	c.Recalculate()
}

// renderMenuBackdrop draws the backdrop's world, if it's loaded. Menu
// screens call it from Render before their UI; drawMenuSky then lays the
// sky or the shot's overlays in the UI pass.
func renderMenuBackdrop(r *render.Renderer) {
	b := menuBD
	if b == nil {
		return
	}
	b.aim(r.Camera)
	r.WeatherOverlay = int(b.sim.Weather.Today().State)
	r.CanopySnow = render.CanopySnowFor(r.WeatherOverlay)
	r.Lighting = render.SunLighting(b.sim.Sun().Dir, r.WeatherOverlay)
	r.DrawWorld(b.world, b.time)
}

// drawMenuSky, in the UI pass: the winter sky while the backdrop loads,
// or over the live backdrop a shade behind the title and the fade
// through the sky at each cut.
func drawMenuSky(r *render.Renderer) {
	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())
	if menuBD == nil {
		r.DrawGradientRect(0, 0, sw, sh, menuSkyTop, menuSkyLow)
		return
	}
	// A shade down from the top, so the title reads over a bright sky:
	// its alpha easing out (quadratically) over the top 40%, in pieces
	// that meet edge to edge, each blended smoothly.
	const pieces = 12
	shade := func(u float32) mgl32.Vec4 {
		c := menuSkyTop
		c[3] = 0.55 * (1 - u) * (1 - u)
		return c
	}
	for i := range pieces {
		u0, u1 := float32(i)/pieces, float32(i+1)/pieces
		r.DrawGradientRect(0, sh*0.4*u0, sw, sh*0.4*(u1-u0), shade(u0), shade(u1))
	}
	t := menuBD.shotT
	if f := max(1-t/menuShotFade, 1-(menuShotLen-t)/menuShotFade); f > 0 {
		c := menuSkyTop
		c[3] = min(f, 1)
		r.DrawColorRect(0, 0, sw, sh, c)
	}
}
