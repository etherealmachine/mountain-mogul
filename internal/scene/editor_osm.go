package scene

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/geo"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// osmOverlay is the editor's OpenStreetMap overlay: the base's lifts,
// runs, roads, water, and ski-area boundary drawn as ribbons draped on
// the ground, with labels, for lining up what's built with the real place.
// It only draws, under everything built; nothing in the world changes. Data © OpenStreetMap
// contributors, ODbL.
type osmOverlay struct {
	shown  bool
	labels []osmLabel
	built  time.Time // when the ribbons were last draped
}

// osmLabel is a feature's name, anchored on the ground at (x, z).
type osmLabel struct {
	x, z float32
	text string
	col  mgl32.Vec4
	rank int // lower draws first and wins where labels overlap
}

// The overlay's look: ribbon widths in metres, how far above the snow
// they float, and how often they're sampled along a path.
const (
	osmLift       = 0.8 // metres above the snow
	osmStep       = 4.0
	osmLiftWidth  = 3.0
	osmRunWidth   = 3.0
	osmAreaWidth  = 3.0
	osmMinRoad    = 3.0
	osmStationBox = 12.0
	// osmRedrape is how often the ribbons are draped again while shown,
	// so they follow the ground and snow as layers and brushes change it.
	osmRedrape = 500 * time.Millisecond
)

var (
	osmWaterCol = mgl32.Vec3{0.05, 0.85, 0.9}  // cyan, apart from blue runs
	osmDryCol   = mgl32.Vec3{0.42, 0.52, 0.66} // dry gullies above where a stream starts
	osmRoadCol  = mgl32.Vec3{1.0, 0.78, 0.25}
	osmLiftCol  = mgl32.Vec3{0.95, 0.15, 0.2}
	osmAreaCol  = mgl32.Vec3{0.75, 0.4, 1.0}
)

// osmFlowWidth is how wide a flowing stream draws, in metres, by the
// catchment that drains through it: 2 m where it starts, wider downstream.
func osmFlowWidth(catchment float32) float32 {
	return min(2+1.5*float32(math.Sqrt(float64(catchment)/1e6)), 10)
}

// osmRunCol is a run's colour by piste:difficulty, as on a trail map.
func osmRunCol(difficulty string) mgl32.Vec3 {
	switch difficulty {
	case "novice", "easy":
		return mgl32.Vec3{0.15, 0.75, 0.3}
	case "intermediate":
		return mgl32.Vec3{0.2, 0.45, 1.0}
	case "advanced", "expert":
		return mgl32.Vec3{0.08, 0.08, 0.1}
	case "freeride", "extreme":
		return mgl32.Vec3{1.0, 0.5, 0.1}
	}
	return mgl32.Vec3{0.6, 0.6, 0.65}
}

// osmCounts sums up what the base has, for the panel row, or says why
// there's nothing.
func osmCounts(base *world.TerrainBase) string {
	if len(base.Lifts)+len(base.Runs)+len(base.Roads)+len(base.Areas)+len(base.Streams)+len(base.Lakes) == 0 {
		if strings.HasPrefix(base.RoadNote, "couldn't fetch") {
			return "not fetched"
		}
		return "nothing mapped"
	}
	var parts []string
	add := func(n int, one, many string) {
		switch {
		case n == 1:
			parts = append(parts, "1 "+one)
		case n > 1:
			parts = append(parts, fmt.Sprintf("%d %s", n, many))
		}
	}
	add(len(base.Lifts), "lift", "lifts")
	add(len(base.Runs), "run", "runs")
	add(len(base.Areas), "ski area", "ski areas")
	add(len(base.Roads), "road", "roads")
	add(len(base.Streams), "stream", "streams")
	add(len(base.Lakes), "lake", "lakes")
	return strings.Join(parts, ", ")
}

// toggleOSMOverlay shows or hides the overlay.
func (e *Editor) toggleOSMOverlay() {
	o := &e.osm
	o.shown = !o.shown
	o.built = time.Time{}
	if !o.shown && e.app != nil && e.app.Renderer != nil {
		e.app.Renderer.SetOverlayVerts(nil)
		o.labels = nil
	}
}

// ShowOSMOverlay turns the overlay on and closes the Layers panel, so the
// map is in full view. For screenshots.
func (e *Editor) ShowOSMOverlay() {
	if !e.osm.shown {
		e.toggleOSMOverlay()
	}
	if e.layers.open {
		e.toggleLayersPanel()
	}
}

// SurfaceAt is the snow surface's height at world (x, z). For
// screenshots.
func (e *Editor) SurfaceAt(x, z float32) float32 {
	if e.world == nil || e.world.Terrain == nil {
		return 0
	}
	return e.world.Terrain.InterpolatedSurfaceElevationAt(x, z)
}

// updateOSMOverlay drapes the ribbons again when they're due.
func (e *Editor) updateOSMOverlay(r *render.Renderer) {
	o := &e.osm
	base := e.world.TerrainBase
	if !o.shown || base == nil {
		if o.shown {
			o.shown = false
			r.SetOverlayVerts(nil)
			o.labels = nil
		}
		return
	}
	if time.Since(o.built) < osmRedrape {
		return
	}
	o.built = time.Now()
	verts, labels := buildOSMOverlay(e.world.Terrain, base, e.layerCache.streamsFor(e.world))
	r.SetOverlayVerts(verts)
	o.labels = labels
}

// osmDraper turns the base's (lat, lon) paths into ribbons on t.
type osmDraper struct {
	t            *world.Terrain
	b            world.GeoBounds
	maxX, maxZ   float32
	verts        []float32
	labels       []osmLabel
	labelledName map[string]bool
}

func buildOSMOverlay(t *world.Terrain, base *world.TerrainBase, streams []geo.Stream) ([]float32, []osmLabel) {
	d := &osmDraper{
		t: t, b: base.Geo,
		maxX:         float32(t.Width-1) * world.CellSize,
		maxZ:         float32(t.Height-1) * world.CellSize,
		labelledName: map[string]bool{},
	}
	for _, a := range base.Areas {
		longest := -1
		for k, p := range a.Paths {
			d.ribbon(p, osmAreaWidth, osmAreaCol)
			if longest < 0 || len(p) > len(a.Paths[longest]) {
				longest = k
			}
		}
		if a.Name != "" && longest >= 0 {
			d.label(a.Paths[longest], a.Name, osmAreaCol, 4)
		}
	}
	// Water first, so roads and lifts draw over it where they cross.
	for _, l := range base.Lakes {
		longest := -1
		for k, p := range l.Paths {
			d.ribbon(p, osmAreaWidth, osmWaterCol)
			if longest < 0 || len(p) > len(l.Paths[longest]) {
				longest = k
			}
		}
		if l.Name != "" && longest >= 0 {
			d.label(l.Paths[longest], l.Name, osmWaterCol, 2)
		}
	}
	// Streams as traced onto their channels: solid where enough ground
	// drains through them to flow, thin and pale above that, where the
	// mapped line runs up a dry gully. Each named stream is labelled once,
	// at the middle of its longest flowing stretch.
	type run struct {
		name   string
		mid    [2]float32
		length float32
	}
	best := map[string]run{}
	for _, s := range streams {
		var runLen float32
		runStart := -1
		for i := 1; i < len(s.Points); i++ {
			a, b := s.Points[i-1], s.Points[i]
			if s.Flowing(i) {
				d.segment(a[0], a[1], b[0], b[1], osmFlowWidth(s.Catchment[i]), osmWaterCol)
				if runStart < 0 {
					runStart, runLen = i-1, 0
				}
				runLen += float32(math.Hypot(float64(b[0]-a[0]), float64(b[1]-a[1])))
			} else {
				d.segment(a[0], a[1], b[0], b[1], 1.5, osmDryCol)
				runStart = -1
			}
			if runStart >= 0 && s.Name != "" && runLen > best[s.Name].length {
				m := s.Points[(runStart+i)/2]
				best[s.Name] = run{s.Name, m, runLen}
			}
		}
	}
	for _, r := range best {
		d.labels = append(d.labels, osmLabel{x: r.mid[0], z: r.mid[1], text: r.name,
			col: mgl32.Vec4{osmWaterCol[0], osmWaterCol[1], osmWaterCol[2], 1}, rank: 3})
	}
	// Roads: one label per name, on its longest piece in the map.
	longestRoad := map[string]int{}
	for k, r := range base.Roads {
		w := max(r.Width, osmMinRoad)
		if r.Tunnel {
			continue
		}
		d.ribbon(r.Path, w, osmRoadCol)
		if r.Name == "" {
			continue
		}
		if j, ok := longestRoad[r.Name]; !ok || d.inLength(r.Path) > d.inLength(base.Roads[j].Path) {
			longestRoad[r.Name] = k
		}
	}
	for name, k := range longestRoad {
		d.label(base.Roads[k].Path, name, osmRoadCol, 2)
	}
	for _, r := range base.Runs {
		col := osmRunCol(r.Difficulty)
		d.ribbon(r.Path, osmRunWidth, col)
		if r.Name != "" {
			d.label(r.Path, r.Name, col, 1)
		}
	}
	for _, l := range base.Lifts {
		if len(l.Path) < 2 {
			continue
		}
		d.ribbon(l.Path, osmLiftWidth, osmLiftCol)
		for _, end := range [][2]float64{l.Path[0], l.Path[len(l.Path)-1]} {
			d.station(end)
		}
		d.label(l.Path, osmLiftName(l), osmLiftCol, 0)
	}
	return d.verts, d.labels
}

// osmLiftName is a lift's label: its name and what kind of lift it is.
func osmLiftName(l world.BaseLift) string {
	kind := strings.ReplaceAll(l.Kind, "_", " ")
	switch l.Kind {
	case "chair_lift":
		kind = "chair"
		switch l.Seats {
		case 0:
		case 2:
			kind = "double chair"
		case 3:
			kind = "triple chair"
		case 4:
			kind = "quad chair"
		case 6:
			kind = "six-pack"
		default:
			kind = fmt.Sprintf("%d-seat chair", l.Seats)
		}
	case "mixed_lift":
		kind = "chair and gondola"
	case "cable_car":
		kind = "tram"
	case "t-bar":
		kind = "T-bar"
	case "j-bar":
		kind = "J-bar"
	case "drag_lift":
		kind = "surface lift"
	}
	if l.Name == "" {
		return kind
	}
	return l.Name + " (" + kind + ")"
}

// toWorld is (lat, lon)'s world position, and whether it's on the map.
func (d *osmDraper) toWorld(p [2]float64) (x, z float32, in bool) {
	x = float32((p[1] - d.b.MinLon) / (d.b.MaxLon - d.b.MinLon) * float64(d.maxX))
	z = float32((d.b.MaxLat - p[0]) / (d.b.MaxLat - d.b.MinLat) * float64(d.maxZ))
	return x, z, x >= 0 && z >= 0 && x <= d.maxX && z <= d.maxZ
}

// y is the drawn surface at (x, z) plus the overlay's float: the mesh
// ground, its lidar detail, and the snow on it.
func (d *osmDraper) y(x, z float32) float32 {
	t := d.t
	snow := t.InterpolatedSurfaceElevationAt(x, z) - t.InterpolatedGroundElevationAt(x, z)
	return t.MeshGroundAt(x/world.CellSize, z/world.CellSize) + t.DetailAt(x, z) + max(snow, 0) + osmLift
}

func (d *osmDraper) vert(x, z float32, col mgl32.Vec3) {
	d.verts = append(d.verts, x, d.y(x, z), z, col[0], col[1], col[2])
}

// ribbon drapes a w-metre ribbon along path, leaving out what's off
// the map.
func (d *osmDraper) ribbon(path [][2]float64, w float32, col mgl32.Vec3) {
	for s := 1; s < len(path); s++ {
		x0, z0, _ := d.toWorld(path[s-1])
		x1, z1, _ := d.toWorld(path[s])
		d.segment(x0, z0, x1, z1, w, col)
	}
}

// segment drapes a w-metre ribbon from world (x0, z0) to (x1, z1),
// leaving out what's off the map.
func (d *osmDraper) segment(x0, z0, x1, z1, w float32, col mgl32.Vec3) {
	half := w / 2
	dx, dz := x1-x0, z1-z0
	l := float32(math.Hypot(float64(dx), float64(dz)))
	if l < 1e-3 || !d.crosses(x0, z0, x1, z1) {
		return
	}
	nx, nz := -dz/l*half, dx/l*half
	n := max(int(math.Ceil(float64(l/osmStep))), 1)
	// Overlap pieces by half a width, so bends don't show gaps.
	over := half / l
	for k := 0; k < n; k++ {
		t0 := max(float32(k)/float32(n)-over, 0)
		t1 := min(float32(k+1)/float32(n)+over, 1)
		ax, az := x0+dx*t0, z0+dz*t0
		bx, bz := x0+dx*t1, z0+dz*t1
		if !d.inside(ax, az) || !d.inside(bx, bz) {
			continue
		}
		d.quad(ax+nx, az+nz, ax-nx, az-nz, bx-nx, bz-nz, bx+nx, bz+nz, col)
	}
}

// crosses reports whether the segment's box overlaps the map.
func (d *osmDraper) crosses(x0, z0, x1, z1 float32) bool {
	return max(x0, x1) >= 0 && min(x0, x1) <= d.maxX && max(z0, z1) >= 0 && min(z0, z1) <= d.maxZ
}

func (d *osmDraper) inside(x, z float32) bool {
	return x >= 0 && z >= 0 && x <= d.maxX && z <= d.maxZ
}

// quad adds a four-cornered piece, each corner on the ground beneath it.
func (d *osmDraper) quad(ax, az, bx, bz, cx, cz, ex, ez float32, col mgl32.Vec3) {
	for _, c := range [6][2]float32{{ax, az}, {bx, bz}, {cx, cz}, {ax, az}, {cx, cz}, {ex, ez}} {
		x := min(max(c[0], 0), d.maxX)
		z := min(max(c[1], 0), d.maxZ)
		d.vert(x, z, col)
	}
}

// station marks a lift's end with a square pad.
func (d *osmDraper) station(p [2]float64) {
	x, z, in := d.toWorld(p)
	if !in {
		return
	}
	h := float32(osmStationBox / 2)
	d.quad(x-h, z-h, x-h, z+h, x+h, z+h, x+h, z-h, osmLiftCol)
}

// inLength is how much of path, in metres, lies on the map.
func (d *osmDraper) inLength(path [][2]float64) float32 {
	var sum float32
	for s := 1; s < len(path); s++ {
		x0, z0, in0 := d.toWorld(path[s-1])
		x1, z1, in1 := d.toWorld(path[s])
		if in0 && in1 {
			sum += float32(math.Hypot(float64(x1-x0), float64(z1-z0)))
		}
	}
	return sum
}

// label anchors text halfway along the part of path on the map.
func (d *osmDraper) label(path [][2]float64, text string, col mgl32.Vec3, rank int) {
	half := d.inLength(path) / 2
	if half <= 0 {
		return
	}
	for s := 1; s < len(path); s++ {
		x0, z0, in0 := d.toWorld(path[s-1])
		x1, z1, in1 := d.toWorld(path[s])
		if !in0 || !in1 {
			continue
		}
		l := float32(math.Hypot(float64(x1-x0), float64(z1-z0)))
		if l >= half {
			f := half / l
			d.labels = append(d.labels, osmLabel{
				x: x0 + (x1-x0)*f, z: z0 + (z1-z0)*f, text: text,
				col: mgl32.Vec4{col[0], col[1], col[2], 1}, rank: rank,
			})
			return
		}
		half -= l
	}
}

// drawOSMLabels draws the overlay's labels, lifts first, skipping any
// that would overlap one already drawn.
func (e *Editor) drawOSMLabels(r *render.Renderer) {
	o := &e.osm
	if !o.shown || r.Font == nil || len(o.labels) == 0 {
		return
	}
	t := e.world.Terrain
	f := r.Font
	type rect struct{ x0, y0, x1, y1 float32 }
	var taken []rect
	for rank := 0; rank <= 4; rank++ {
		for _, l := range o.labels {
			if l.rank != rank {
				continue
			}
			y := t.InterpolatedSurfaceElevationAt(l.x, l.z) + t.DetailAt(l.x, l.z) + 6
			sx, sy, vis := r.WorldToScreen(mgl32.Vec3{l.x, y, l.z})
			if !vis {
				continue
			}
			tw := f.TextWidth(l.text)
			gh := float32(render.GlyphH)
			box := rect{sx - tw/2 - 5, sy - gh/2 - 2, sx + tw/2 + 5, sy + gh/2 + 2}
			clash := false
			for _, b := range taken {
				if box.x0 < b.x1 && box.x1 > b.x0 && box.y0 < b.y1 && box.y1 > b.y0 {
					clash = true
					break
				}
			}
			if clash {
				continue
			}
			taken = append(taken, box)
			r.DrawColorRect(box.x0, box.y0, box.x1-box.x0, box.y1-box.y0, mgl32.Vec4{0.05, 0.06, 0.09, 0.8})
			r.DrawColorRect(box.x0, box.y0, 3, box.y1-box.y0, l.col)
			text := mgl32.Vec4{0.95, 0.96, 1, 1}
			f.DrawText(r, l.text, sx-tw/2+1, sy-gh/2, text)
		}
	}
}
