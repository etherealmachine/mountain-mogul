package scene

import (
	"context"
	"fmt"
	"maps"
	"math"
	"sync"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/geo"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

type tisState int

const (
	tisSearch tisState = iota
	tisSearching
	tisResults
	tisMap
	tisFetching
)

// importMetersPerCell mirrors the engine-wide cell size. The selection
// square's pixel extent is derived from this so the imported region
// always satisfies cell == 5 m on the ground regardless of preview zoom.
const importMetersPerCell = 5.0

// Import grid-size limits. Min keeps the imported map at least a few
// hundred metres on a side so chosen regions aren't useless; max is
// the historical "Large" preset so saves and renderer sizing don't
// inherit larger-than-tested maps. Default size sits in the middle.
const (
	importMinCells     = 128
	importMaxCells     = 1280
	importDefaultCells = 512
)

// importCornerHandlePx is the screen-space click target for each
// selection-square corner. Generous so a corner is easy to grab on a
// preview that may have small pixel dimensions for tight crops.
const importCornerHandlePx = float32(18)

// tiJob handles any single background network operation for the terrain import scene.
type tiJob struct {
	mu            sync.Mutex
	searchResults []geo.SearchResult
	mapResult     *geo.PreviewResult
	imported      *ImportedTerrain
	stage         string
	progress      float32
	err           error
	done          bool
	cancel        context.CancelFunc
}

func (j *tiJob) setStage(stage string, p float32) {
	j.mu.Lock()
	j.stage, j.progress = stage, p
	j.mu.Unlock()
}

func (j *tiJob) stageProgress() (string, float32) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.stage, j.progress
}

func (j *tiJob) finish(err error) {
	j.mu.Lock()
	j.err = err
	j.done = true
	j.mu.Unlock()
}

func (j *tiJob) snapshot() (progress float32, sr []geo.SearchResult, mr *geo.PreviewResult, imp *ImportedTerrain, err error, done bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.progress, j.searchResults, j.mapResult, j.imported, j.err, j.done
}

// ImportedTerrain is a finished import: the world built from it, and
// the layer stack that made its ground from the surveyed base.
type ImportedTerrain struct {
	Result *geo.ImportResult
	World  *world.World
	Layers *geo.LayerStack
	cache  layerCache
}

// TerrainImport is a full-screen scene for searching, previewing, and importing
// real-world elevation data into the scenario editor terrain.
type TerrainImport struct {
	app      *engine.App
	onImport func(ImportedTerrain)

	// reopen, when set, starts on the map framed on this square instead
	// of the search box. layersOff and strengths carry over which layers
	// were off and how strong each was.
	reopen    *world.GeoBounds
	layersOff []string
	strengths map[string]float32

	// gridSize is the destination grid's side length. The selection
	// square covers gridSize × importMetersPerCell metres of ground;
	// the elevation fetch resamples to gridSize × gridSize.
	gridSize int

	state     tisState
	searchBuf string
	errMsg    string

	searchResults  []geo.SearchResult
	selectedResult *geo.SearchResult

	// interactive map
	mapTexID   uint32
	mapTexW    float32
	mapTexH    float32
	mapBounds  [4]float64 // [minLat, maxLat, minLon, maxLon] of current texture
	mapCenter  [2]float64 // [lat, lon]
	mapZoom    int
	mapScale   float32 // visual scale multiplier; crosses 2×/0.5× to step tile zoom level
	mapLoading bool

	// left-drag pan — panAccum persists across drags; only resets on tile reload
	panAccum  mgl32.Vec2
	panDrag   mgl32.Vec2
	panActive bool

	// Selection-square corner drag: active while the user is resizing
	// the imported area by dragging one of its corner handles. Takes
	// priority over panActive so a corner-grab doesn't also pan.
	resizeActive bool

	job     *tiJob
	menuBar *ui.MenuBar

	// OpenStreetMap lifts, runs and ski-area boundaries drawn over the
	// preview. osmAsked lists every box fetched so far, so the view is
	// only refetched once it leaves them.
	osm        geo.OSMMap
	osmAsked   []geo.Bounds
	osmJob     *osmJob
	osmHidden  bool
	osmNote    string
	osmRetryAt time.Time
}

// osmMaxSpanDeg caps an overlay request's side so a zoomed-out view
// doesn't ask Overpass for a whole mountain range.
const osmMaxSpanDeg = 0.35

// osmMinZoom is the coarsest preview zoom that loads the overlay.
const osmMinZoom = 11

type osmJob struct {
	mu     sync.Mutex
	asked  geo.Bounds
	res    *geo.OSMMap
	err    error
	done   bool
	cancel context.CancelFunc
}

// NewTerrainImport creates the scene.
// initialGridSize is the starting destination grid side length; the
// player resizes the imported area inside the map view by dragging the
// selection-square corners. onImport is called with the finished import
// on success.
func NewTerrainImport(initialGridSize int, onImport func(ImportedTerrain)) *TerrainImport {
	g := initialGridSize
	if g <= 0 {
		g = importDefaultCells
	}
	if g < importMinCells {
		g = importMinCells
	}
	if g > importMaxCells {
		g = importMaxCells
	}
	return &TerrainImport{
		onImport: onImport,
		gridSize: g,
		state:    tisSearch,
		mapZoom:  12,
		mapScale: 1.0,
	}
}

// NewTerrainReimport opens the import on base's square, at the map's
// grid size, keeping which layers were off and their strengths.
func NewTerrainReimport(gridSize int, base *world.TerrainBase, onImport func(ImportedTerrain)) *TerrainImport {
	t := NewTerrainImport(gridSize, onImport)
	g := base.Geo
	t.reopen = &g
	t.layersOff = append([]string(nil), base.LayersOff...)
	t.strengths = maps.Clone(base.Strengths)
	return t
}

func (t *TerrainImport) Init(app *engine.App) error {
	t.app = app
	t.menuBar = ui.NewMenuBar(0, 32)
	t.menuBar.AddButton("Import Terrain", func() {}) // title label
	t.menuBar.AddButton("Back", t.goBack)
	t.menuBar.AddButton("Cancel", func() { app.PopScene() })
	if g := t.reopen; g != nil {
		lat, lon := (g.MinLat+g.MaxLat)/2, (g.MinLon+g.MaxLon)/2
		t.PreviewAt(lat, lon, reopenZoom(lat, t.gridSize, app.Renderer.ScreenHeight()-32))
	}
	return nil
}

// reopenZoom is the closest preview zoom at which a gridSize-cell square
// at lat fills no more than two thirds of a viewH-pixel-tall map.
func reopenZoom(lat float64, gridSize, viewH int) int {
	side := float64(gridSize) * importMetersPerCell
	cosLat := max(math.Cos(lat*math.Pi/180), 0.05)
	for z := 16; z > 9; z-- {
		if side/(156543.0339*cosLat/math.Pow(2, float64(z))) <= float64(viewH)*2/3 {
			return z
		}
	}
	return 9
}

// goBack steps back one screen: a running fetch to the map, the map to
// the results (or the search box when the search had a single result),
// the results to the search box, and the search box out of the import.
func (t *TerrainImport) goBack() {
	if t.job != nil {
		t.job.cancel()
		t.job = nil
	}
	t.mapLoading = false
	t.resizeActive, t.panActive = false, false
	t.errMsg = ""
	switch t.state {
	case tisFetching:
		t.state = tisMap
	case tisMap:
		if len(t.searchResults) > 1 {
			t.state = tisResults
		} else {
			t.state = tisSearch
		}
	case tisResults, tisSearching:
		t.state = tisSearch
	default:
		t.app.PopScene()
	}
}

func (t *TerrainImport) Update(dt float64) {
	inp := t.app.Input
	r := t.app.Renderer

	t.menuBar.HandleInput(inp, float32(r.ScreenWidth()), float32(r.ScreenHeight()))
	if inp.Pressed[glfw.KeyEscape] {
		t.goBack()
		return
	}

	switch t.state {
	case tisSearch:
		for _, ch := range inp.CharInput {
			t.searchBuf += string(ch)
		}
		if inp.Pressed[glfw.KeyBackspace] && len(t.searchBuf) > 0 {
			runes := []rune(t.searchBuf)
			t.searchBuf = string(runes[:len(runes)-1])
		}
		if (inp.Pressed[glfw.KeyEnter] || inp.Pressed[glfw.KeyKPEnter]) && len(t.searchBuf) > 0 {
			t.startSearch()
		}

	case tisSearching:
		if t.job == nil {
			break
		}
		_, sr, _, _, err, done := t.job.snapshot()
		if !done {
			break
		}
		t.job = nil
		switch {
		case err != nil:
			t.errMsg = "Error: " + err.Error()
			t.state = tisSearch
		case len(sr) == 0:
			t.errMsg = fmt.Sprintf("No places found for %q. Try the resort's full name, e.g. \"Boreal Mountain Resort\".", t.searchBuf)
			t.state = tisSearch
		case len(sr) == 1:
			t.searchResults = sr
			res := sr[0]
			t.selectedResult = &res
			t.loadMapFromResult()
		default:
			t.searchResults = sr
			t.state = tisResults
		}

	case tisResults:
		sw := float32(r.ScreenWidth())
		sh := float32(r.ScreenHeight())
		const rowH = float32(44)
		listX := sw/2 - 280
		listY := sh/2 - float32(len(t.searchResults))*rowH/2
		if inp.LeftClick {
			for i, res := range t.searchResults {
				rowY := listY + float32(i)*rowH
				if inp.MousePos[0] >= listX && inp.MousePos[0] <= listX+560 &&
					inp.MousePos[1] >= rowY && inp.MousePos[1] <= rowY+rowH-2 {
					res2 := res
					t.selectedResult = &res2
					t.loadMapFromResult()
					break
				}
			}
		}

	case tisMap:
		t.updateMap(inp, r)

	case tisFetching:
		if t.job == nil {
			break
		}
		_, _, _, imp, err, done := t.job.snapshot()
		if !done {
			break
		}
		t.job = nil
		if err != nil {
			t.errMsg = err.Error()
			t.state = tisMap
		} else {
			t.onImport(*imp)
			t.app.PopScene()
		}
	}
}

func (t *TerrainImport) updateMap(inp *engine.Input, r *render.Renderer) {
	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())
	mx := inp.MousePos[0]
	my := inp.MousePos[1]
	inMap := my > 32

	// Poll the map-reload job; upload texture on the main thread when ready.
	if t.mapLoading && t.job != nil {
		_, _, mr, _, err, done := t.job.snapshot()
		if done {
			t.job = nil
			t.mapLoading = false
			if err != nil {
				t.errMsg = err.Error()
			} else if mr != nil {
				t.uploadMapTexture(mr)
			}
		}
	}

	if inp.Pressed[glfw.KeyO] {
		t.osmHidden = !t.osmHidden
	}
	t.updateOSM(sw, sh)

	// ── Corner-drag: resize selection square ─────────────────────────────────
	// Tested before pan so grabbing a corner handle doesn't also start
	// dragging the map. Only fires while the map is shown and a tile
	// has been loaded.
	if inp.LeftClick && inMap && !t.mapLoading && t.mapTexID != 0 && !t.resizeActive {
		if t.cornerHit(mx, my, sw, sh) {
			t.resizeActive = true
		}
	}
	if t.resizeActive && inp.LeftHeld {
		t.applyResizeDrag(mx, my, sw, sh)
	}
	if t.resizeActive && inp.LeftRelease {
		t.resizeActive = false
	}

	// ── Left drag: pan ───────────────────────────────────────────────────────
	if inp.LeftClick && inMap && !t.mapLoading && !t.resizeActive {
		t.panActive = true
		t.panDrag = mgl32.Vec2{}
	}
	if t.panActive && inp.LeftHeld {
		t.panDrag = t.panDrag.Add(inp.MouseDelta)
	}
	if t.panActive && inp.LeftRelease {
		t.panActive = false
		t.panAccum = t.panAccum.Add(t.panDrag)
		t.panDrag = mgl32.Vec2{}
		t.refreshMapCenter()
	}

	// ── Scroll: smooth zoom ──────────────────────────────────────────────────
	if inp.ScrollDelta != 0 && inMap {
		t.mapScale *= float32(math.Pow(1.15, float64(inp.ScrollDelta)))

		// When scale crosses 2× or 0.5×, step the tile zoom level and fetch
		// new tiles. Before the reload, bake panAccum into mapCenter so the new
		// tile is centered on whatever is currently at screen center.
		if !t.mapLoading {
			if t.mapScale >= 2.0 && t.mapZoom < 18 {
				t.mapZoom++
				t.mapScale /= 2.0
				t.refreshMapCenter()
				t.panAccum = mgl32.Vec2{}
				t.startMapReload(int(sw), int(sh-32))
			} else if t.mapScale <= 0.5 && t.mapZoom > 3 {
				t.mapZoom--
				t.mapScale *= 2.0
				t.refreshMapCenter()
				t.panAccum = mgl32.Vec2{}
				t.startMapReload(int(sw), int(sh-32))
			}
		}

		// Hard clamp at zoom level limits so scale doesn't drift unbounded.
		if t.mapZoom >= 18 && t.mapScale > 3.0 {
			t.mapScale = 3.0
		}
		if t.mapZoom <= 3 && t.mapScale < 0.33 {
			t.mapScale = 0.33
		}

		// If the current tile no longer covers the viewport at the new scale
		// (i.e. we zoomed out but haven't crossed the 0.5 threshold yet),
		// reload at the same zoom level with an expanded request so the
		// new tile fills the screen rather than leaving black edges.
		if t.mapTexID != 0 && !t.mapLoading &&
			(t.mapTexW*t.mapScale < sw || t.mapTexH*t.mapScale < sh-32) {
			t.refreshMapCenter()
			t.panAccum = mgl32.Vec2{}
			reqW := int(math.Ceil(float64(sw) / float64(t.mapScale)))
			reqH := int(math.Ceil(float64(sh-32) / float64(t.mapScale)))
			t.startMapReload(reqW, reqH)
		}
	}

	// ── Import button ────────────────────────────────────────────────────────
	if t.mapTexID != 0 && inp.LeftClick && !t.resizeActive {
		btnX := sw - 140
		btnY := sh - 50
		if mx >= btnX && mx <= btnX+130 && my >= btnY && my <= btnY+36 {
			t.startFetch()
		}
	}
}

// cornerHit reports whether the given screen point is within the
// click target of any of the selection square's four corners.
func (t *TerrainImport) cornerHit(mx, my, sw, sh float32) bool {
	x0, y0, x1, y1 := t.selectionSquare(sw, sh)
	corners := [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}}
	hp := importCornerHandlePx / 2
	for _, c := range corners {
		if mx >= c[0]-hp && mx <= c[0]+hp && my >= c[1]-hp && my <= c[1]+hp {
			return true
		}
	}
	return false
}

// applyResizeDrag converts the cursor's distance from the selection
// square centre into a new gridSize. The square stays centred on
// screen, so dragging a corner outward grows it symmetrically and
// dragging inward shrinks it. Sized as max(|dx|, |dy|) from centre to
// keep the shape square regardless of which axis the cursor moves
// along most.
func (t *TerrainImport) applyResizeDrag(mx, my, sw, sh float32) {
	cx := sw / 2
	cy := 32 + (sh-32)/2
	dx := mx - cx
	if dx < 0 {
		dx = -dx
	}
	dy := my - cy
	if dy < 0 {
		dy = -dy
	}
	half := dx
	if dy > half {
		half = dy
	}
	gs := t.gridSizeForHalfPx(half)
	if gs < importMinCells {
		gs = importMinCells
	}
	if gs > importMaxCells {
		gs = importMaxCells
	}
	t.gridSize = gs
}

// gridSizeForHalfPx inverts selectionHalfPx — given a pixel half-size
// for the current preview, returns the corresponding grid side length
// in cells.
func (t *TerrainImport) gridSizeForHalfPx(halfPx float32) int {
	cosLat := math.Cos(t.mapCenter[0] * math.Pi / 180)
	if cosLat < 0.05 {
		cosLat = 0.05
	}
	mppNative := 156543.0339 * cosLat / math.Pow(2, float64(t.mapZoom))
	mpp := mppNative / float64(t.mapScale)
	if mpp <= 0 {
		mpp = 1
	}
	groundMetres := float64(halfPx) * 2 * mpp
	return int(groundMetres / importMetersPerCell)
}

// refreshMapCenter updates mapCenter to the geographic point currently at screen
// centre, accounting for accumulated pan. Called before any tile reload so the
// new tile is centred on the right location.
func (t *TerrainImport) refreshMapCenter() {
	if t.mapTexW == 0 || t.mapTexH == 0 {
		return
	}
	drawW := float64(t.mapTexW * t.mapScale)
	drawH := float64(t.mapTexH * t.mapScale)
	fx := 0.5 - float64(t.panAccum[0])/drawW
	fy := 0.5 - float64(t.panAccum[1])/drawH
	t.mapCenter[0], t.mapCenter[1] = t.texLatLon(fx, fy)
}

// mapRect is where the preview texture is drawn on screen.
func (t *TerrainImport) mapRect(sw, sh float32) (x, y, w, h float32) {
	pan := t.panAccum.Add(t.panDrag)
	w = t.mapTexW * t.mapScale
	h = t.mapTexH * t.mapScale
	return sw/2 - w/2 + pan[0], 32 + (sh-32)/2 - h/2 + pan[1], w, h
}

// texLatLon maps a fraction across the preview texture (0,0 top-left)
// to lat/lon. The tiles are Web Mercator: linear in longitude and in
// Mercator northing, not in latitude.
func (t *TerrainImport) texLatLon(fx, fy float64) (lat, lon float64) {
	yTop, yBot := geo.MercatorY(t.mapBounds[1]), geo.MercatorY(t.mapBounds[0])
	lat = geo.MercatorLat(yTop - fy*(yTop-yBot))
	lon = t.mapBounds[2] + fx*(t.mapBounds[3]-t.mapBounds[2])
	return lat, lon
}

func (t *TerrainImport) screenLatLon(sx, sy, sw, sh float32) (lat, lon float64) {
	x, y, w, h := t.mapRect(sw, sh)
	return t.texLatLon(float64((sx-x)/w), float64((sy-y)/h))
}

// osmProjector precomputes the lat/lon → screen mapping for one frame.
type osmProjector struct {
	x, y, w, h      float64
	minLon, lonSpan float64
	yTop, mercSpan  float64
}

func (t *TerrainImport) projector(sw, sh float32) osmProjector {
	x, y, w, h := t.mapRect(sw, sh)
	yTop := geo.MercatorY(t.mapBounds[1])
	return osmProjector{
		x: float64(x), y: float64(y), w: float64(w), h: float64(h),
		minLon: t.mapBounds[2], lonSpan: t.mapBounds[3] - t.mapBounds[2],
		yTop: yTop, mercSpan: yTop - geo.MercatorY(t.mapBounds[0]),
	}
}

func (p osmProjector) at(ll geo.LatLon) (float32, float32) {
	fx := (ll.Lon - p.minLon) / p.lonSpan
	fy := (p.yTop - geo.MercatorY(ll.Lat)) / p.mercSpan
	return float32(p.x + fx*p.w), float32(p.y + fy*p.h)
}

// visibleBounds is the lat/lon box of the map area on screen.
func (t *TerrainImport) visibleBounds(sw, sh float32) geo.Bounds {
	maxLat, minLon := t.screenLatLon(0, 32, sw, sh)
	minLat, maxLon := t.screenLatLon(sw, sh, sw, sh)
	return geo.Bounds{MinLat: minLat, MaxLat: maxLat, MinLon: minLon, MaxLon: maxLon}
}

// updateOSM collects a finished overlay fetch and starts the next one
// when the visible area has moved outside everything already asked for.
func (t *TerrainImport) updateOSM(sw, sh float32) {
	if j := t.osmJob; j != nil {
		j.mu.Lock()
		done, res, err := j.done, j.res, j.err
		j.mu.Unlock()
		if !done {
			return
		}
		t.osmJob = nil
		if err != nil {
			t.osmNote = "Couldn't load OpenStreetMap lifts and runs; retrying shortly"
			t.osmRetryAt = time.Now().Add(30 * time.Second)
			return
		}
		t.osmAsked = append(t.osmAsked, res.Covered)
		t.osm.Merge(res)
		t.osmNote = ""
		if !res.Covered.Contains(j.asked) {
			t.osmNote = "OpenStreetMap is busy; lifts and runs shown near the centre only"
			t.osmRetryAt = time.Now().Add(30 * time.Second)
		}
	}
	if t.mapTexID == 0 || t.mapLoading || t.panActive || t.state != tisMap || time.Now().Before(t.osmRetryAt) {
		return
	}
	if t.mapZoom < osmMinZoom {
		t.osmNote = "Zoom in to see lifts and runs"
		return
	}
	if t.osmNote == "Zoom in to see lifts and runs" {
		t.osmNote = ""
	}
	want := t.visibleBounds(sw, sh).ClampSpan(osmMaxSpanDeg)
	for _, b := range t.osmAsked {
		if b.Contains(want) {
			return
		}
	}
	// Ask for a margin around the view so small pans don't refetch.
	mLat, mLon := (want.MaxLat-want.MinLat)/4, (want.MaxLon-want.MinLon)/4
	ask := geo.Bounds{MinLat: want.MinLat - mLat, MaxLat: want.MaxLat + mLat,
		MinLon: want.MinLon - mLon, MaxLon: want.MaxLon + mLon}.ClampSpan(osmMaxSpanDeg)

	ctx, cancel := context.WithCancel(context.Background())
	j := &osmJob{asked: ask, cancel: cancel}
	t.osmJob = j
	go func() {
		res, err := geo.FetchOSM(ctx, ask, geo.OSMSki)
		j.mu.Lock()
		j.res, j.err, j.done = res, err, true
		j.mu.Unlock()
	}()
}

func (t *TerrainImport) startMapReload(w, h int) {
	if t.mapLoading {
		if t.job != nil {
			t.job.cancel()
		}
	}
	t.mapLoading = true

	lat, lon, zoom := t.mapCenter[0], t.mapCenter[1], t.mapZoom

	_, cancel := context.WithCancel(context.Background())
	j := &tiJob{cancel: cancel}
	t.job = j

	go func() {
		pr, err := geo.RenderPreviewAt(lat, lon, zoom, w, h)
		j.mu.Lock()
		if err == nil {
			j.mapResult = &pr
		}
		j.err = err
		j.done = true
		j.mu.Unlock()
	}()
}

func (t *TerrainImport) loadMapFromResult() {
	t.state = tisMap
	t.mapScale = 1.0
	t.panAccum = mgl32.Vec2{}
	t.panDrag = mgl32.Vec2{}
	t.errMsg = ""
	if t.mapTexID != 0 {
		gl.DeleteTextures(1, &t.mapTexID)
		t.mapTexID = 0
	}

	res := t.selectedResult
	t.mapCenter[0] = (res.BBox[0] + res.BBox[1]) / 2
	t.mapCenter[1] = (res.BBox[2] + res.BBox[3]) / 2
	latSpan := res.BBox[1] - res.BBox[0]
	switch {
	case latSpan < 0.05:
		t.mapZoom = 15
	case latSpan < 0.2:
		t.mapZoom = 13
	case latSpan < 1.0:
		t.mapZoom = 11
	default:
		t.mapZoom = 9
	}

	sw := t.app.Renderer.ScreenWidth()
	sh := t.app.Renderer.ScreenHeight()
	t.startMapReload(sw, sh-32)
}

// PreviewAt opens the map screen centred on a point, skipping the
// search. Used by -screenshot -import-preview.
func (t *TerrainImport) PreviewAt(lat, lon float64, zoom int) {
	t.state = tisMap
	t.mapCenter = [2]float64{lat, lon}
	t.mapZoom = zoom
	t.startMapReload(t.app.Renderer.ScreenWidth(), t.app.Renderer.ScreenHeight()-32)
}

// Settled reports that the map and the overlay for the current view
// have both finished loading.
func (t *TerrainImport) Settled() bool {
	return t.mapTexID != 0 && !t.mapLoading && t.osmJob == nil
}

func (t *TerrainImport) startSearch() {
	t.state = tisSearching
	t.errMsg = ""
	query := t.searchBuf

	_, cancel := context.WithCancel(context.Background())
	j := &tiJob{cancel: cancel}
	if t.job != nil {
		t.job.cancel()
	}
	t.job = j

	go func() {
		results, err := geo.Search(query)
		j.mu.Lock()
		j.searchResults = results
		j.err = err
		j.done = true
		j.mu.Unlock()
	}()
}

func (t *TerrainImport) startFetch() {
	if t.mapTexID == 0 {
		return
	}
	t.state = tisFetching
	t.errMsg = ""

	// Convert the selection square to lat/lon bounds. Pan is committed
	// before Import can be clicked.
	sw := float32(t.app.Renderer.ScreenWidth())
	sh := float32(t.app.Renderer.ScreenHeight())
	sx0, sy0, sx1, sy1 := t.selectionSquare(sw, sh)

	maxLat, minLon := t.screenLatLon(sx0, sy0, sw, sh)
	minLat, maxLon := t.screenLatLon(sx1, sy1, sw, sh)
	bounds := geo.Bounds{MinLat: minLat, MaxLat: maxLat, MinLon: minLon, MaxLon: maxLon}
	n := t.gridSize
	off, strengths := t.layersOff, t.strengths

	ctx, cancel := context.WithCancel(context.Background())
	j := &tiJob{cancel: cancel}
	if t.job != nil {
		t.job.cancel()
	}
	t.job = j

	go func() {
		imp, err := importSquare(ctx, bounds, n, off, strengths, j.setStage)
		if err != nil {
			j.finish(err)
			return
		}
		j.mu.Lock()
		j.imported = imp
		j.mu.Unlock()
		j.finish(nil)
	}()
}

// importSquare fetches bounds on an n × n grid and builds a world from
// it with these layer settings (off, strengths), reporting each stage.
// Used by the import screen and the Layers panel's reload.
func importSquare(ctx context.Context, bounds geo.Bounds, n int, off []string, strengths map[string]float32, stage func(string, float32)) (*ImportedTerrain, error) {
	res, err := geo.ImportTerrain(ctx, bounds, n, n, world.DetailPerCell, stage)
	if err != nil {
		return nil, err
	}
	w, stack, err := geo.BuildWorld(res, off, strengths, func(layer string) { stage(layer, 0) })
	if err != nil {
		return nil, err
	}
	imp := &ImportedTerrain{Result: res, World: w, Layers: stack}
	dressWorld(w, &imp.cache, func(name string) { stage(name, 0) })
	return imp, nil
}

func (t *TerrainImport) uploadMapTexture(pr *geo.PreviewResult) {
	if t.mapTexID != 0 {
		gl.DeleteTextures(1, &t.mapTexID)
		t.mapTexID = 0
	}
	bounds := pr.Image.Bounds()
	w, h := int32(bounds.Dx()), int32(bounds.Dy())

	var texID uint32
	gl.GenTextures(1, &texID)
	gl.BindTexture(gl.TEXTURE_2D, texID)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pr.Image.Pix))
	gl.BindTexture(gl.TEXTURE_2D, 0)

	t.mapTexID = texID
	t.mapTexW = float32(w)
	t.mapTexH = float32(h)
	t.mapBounds = [4]float64{pr.MinLat, pr.MaxLat, pr.MinLon, pr.MaxLon}
	t.mapCenter[0] = (pr.MinLat + pr.MaxLat) / 2
	t.mapCenter[1] = (pr.MinLon + pr.MaxLon) / 2
	t.panAccum = mgl32.Vec2{} // new tile is centred on mapCenter; no offset needed
}

func (t *TerrainImport) Render(r *render.Renderer) {
	var content render.UIDrawable
	switch t.state {
	case tisMap, tisFetching:
		content = &tiMapDrawable{t}
	default:
		content = &tiSearchDrawable{t}
	}
	r.DrawUI([]render.UIDrawable{t.menuBar, content})
}

func (t *TerrainImport) Destroy() {
	if t.job != nil {
		t.job.cancel()
	}
	if t.osmJob != nil {
		t.osmJob.cancel()
	}
	if t.mapTexID != 0 {
		gl.DeleteTextures(1, &t.mapTexID)
	}
}

// ── UIDrawable implementations ────────────────────────────────────────────────

type tiSearchDrawable struct{ t *TerrainImport }

func (d *tiSearchDrawable) Draw(r *render.Renderer) {
	t := d.t
	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())
	white := mgl32.Vec4{1, 1, 1, 1}
	grey := mgl32.Vec4{0.7, 0.7, 0.7, 1}
	red := mgl32.Vec4{0.9, 0.3, 0.2, 1}

	r.DrawColorRect(0, 32, sw, sh-32, mgl32.Vec4{0.05, 0.08, 0.18, 1})

	if r.Font == nil {
		return
	}

	switch t.state {
	case tisSearch:
		if t.errMsg != "" {
			msg := tiTruncate(t.errMsg, 110)
			r.Font.DrawText(r, msg, sw/2-r.Font.TextWidth(msg)/2, sh/2-80, red)
		}
		r.Font.DrawText(r, "Search for a location:", sw/2-120, sh/2-50, grey)
		fx, fy, fw, fh := sw/2-200, sh/2-26, float32(400), float32(32)
		r.DrawColorRect(fx, fy, fw, fh, mgl32.Vec4{0.05, 0.05, 0.12, 1})
		r.DrawColorRect(fx, fy, fw, 1, mgl32.Vec4{0.5, 0.5, 0.9, 1})
		r.DrawColorRect(fx, fy+fh-1, fw, 1, mgl32.Vec4{0.5, 0.5, 0.9, 1})
		r.DrawColorRect(fx, fy, 1, fh, mgl32.Vec4{0.5, 0.5, 0.9, 1})
		r.DrawColorRect(fx+fw-1, fy, 1, fh, mgl32.Vec4{0.5, 0.5, 0.9, 1})
		r.Font.DrawText(r, t.searchBuf+"_", fx+6, fy+6, white)
		r.Font.DrawText(r, "Press Enter to search   Esc: back", sw/2-150, sh/2+14, grey)

	case tisSearching:
		r.Font.DrawText(r, "Searching...", sw/2-60, sh/2-10, grey)

	case tisResults:
		const rowH = float32(44)
		listX := sw/2 - 280
		listY := sh/2 - float32(len(t.searchResults))*rowH/2
		r.Font.DrawText(r, "Click a result to view map:", listX, listY-28, grey)
		for i, res := range t.searchResults {
			rowY := listY + float32(i)*rowH
			r.DrawColorRect(listX, rowY, 560, rowH-2, mgl32.Vec4{0.15, 0.2, 0.35, 0.9})
			name := res.DisplayName
			if res.SkiArea {
				name = "Ski area: " + name
			}
			r.Font.DrawText(r, tiTruncate(name, 62), listX+8, rowY+12, white)
		}
	}
}

type tiMapDrawable struct{ t *TerrainImport }

func (d *tiMapDrawable) Draw(r *render.Renderer) {
	t := d.t
	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())
	white := mgl32.Vec4{1, 1, 1, 1}
	grey := mgl32.Vec4{0.7, 0.7, 0.7, 1}
	yellow := mgl32.Vec4{1, 0.9, 0.2, 1}

	// Background behind the map (visible when panning near edges)
	r.DrawColorRect(0, 32, sw, sh-32, mgl32.Vec4{0.08, 0.08, 0.1, 1})

	if t.mapTexID != 0 {
		pan := t.panAccum.Add(t.panDrag)
		drawW := t.mapTexW * t.mapScale
		drawH := t.mapTexH * t.mapScale
		drawX := sw/2 - drawW/2 + pan[0]
		drawY := 32 + (sh-32)/2 - drawH/2 + pan[1]
		r.DrawTexturedRect(drawX, drawY, drawW, drawH, t.mapTexID, white)
		if !t.osmHidden {
			d.drawOSM(r, sw, sh)
		}
	}

	if r.Font == nil {
		return
	}

	if t.mapTexID == 0 || t.mapLoading {
		r.Font.DrawText(r, "Loading map...", sw/2-60, sh/2, grey)
	}

	// Selection square — always centred on the map area, sized so its
	// extent in metres equals gridSize × importMetersPerCell. Corner
	// handles let the player resize it by drag.
	if t.mapTexID != 0 {
		qx0, qy0, qx1, qy1 := t.selectionSquare(sw, sh)
		thick := float32(2)
		r.DrawColorRect(qx0, qy0, qx1-qx0, thick, yellow)
		r.DrawColorRect(qx0, qy1-thick, qx1-qx0, thick, yellow)
		r.DrawColorRect(qx0, qy0, thick, qy1-qy0, yellow)
		r.DrawColorRect(qx1-thick, qy0, thick, qy1-qy0, yellow)

		// Corner handles — solid square markers on each corner so the
		// drag target is obvious. Sized to match importCornerHandlePx
		// (the hit-test extent) so what you see is what you can grab.
		hs := importCornerHandlePx / 2
		corners := [4][2]float32{{qx0, qy0}, {qx1, qy0}, {qx0, qy1}, {qx1, qy1}}
		for _, c := range corners {
			r.DrawColorRect(c[0]-hs, c[1]-hs, hs*2, hs*2, yellow)
		}

		// Caption beneath the square: cells × cells (km × km).
		km := float64(t.gridSize) * importMetersPerCell / 1000.0
		caption := fmt.Sprintf("%d × %d cells   (%.2f km × %.2f km)",
			t.gridSize, t.gridSize, km, km)
		captionW := r.Font.TextWidth(caption)
		captionX := (qx0+qx1)/2 - captionW/2
		captionY := qy1 + 6 + hs
		if captionY > sh-44 {
			captionY = qy0 - float32(render.GlyphH) - 6 - hs
		}
		r.Font.DrawText(r, caption, captionX, captionY, yellow)
	}

	// Hint
	if t.mapTexID != 0 && t.state != tisFetching {
		r.Font.DrawText(r, "Left-drag: pan   Scroll: zoom   Drag a corner of the square to resize   O: lifts & runs   Esc: back", 10, sh-22, grey)
		d.drawOSMStatus(r, sw, sh)
	}

	if t.errMsg != "" {
		r.Font.DrawText(r, tiTruncate("Error: "+t.errMsg, 60), 10, sh-44, mgl32.Vec4{0.9, 0.3, 0.2, 1})
	}

	// Import button
	if t.mapTexID != 0 && t.state == tisMap {
		btnX := sw - 140
		btnY := sh - 50
		r.DrawColorRect(btnX, btnY, 130, 36, mgl32.Vec4{0.15, 0.5, 0.2, 0.95})
		r.Font.DrawText(r, "Import!", btnX+14, btnY+8, white)
	}

	// Elevation fetch progress
	if t.state == tisFetching && t.job != nil {
		stage, progress := t.job.stageProgress()
		if stage == "" {
			stage = "Fetching elevation"
		}
		label := fmt.Sprintf("%s... %d%%", stage, int(progress*100))
		r.Font.DrawText(r, label, sw/2-r.Font.TextWidth(label)/2, sh/2-20, white)
		barX, barY, barW, barH := sw/2-200, sh/2+10, float32(400), float32(20)
		r.DrawColorRect(barX, barY, barW, barH, mgl32.Vec4{0.1, 0.1, 0.2, 1})
		r.DrawColorRect(barX, barY, barW*progress, barH, mgl32.Vec4{0.2, 0.6, 0.3, 1})
	}
}

func tiTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// selectionSquare returns the screen corners of the import-area square,
// centred in the map area below the menu bar. The pixel extent is
// derived from the current preview zoom + map centre latitude so the
// square always covers gridSize × importMetersPerCell metres of
// ground — i.e., zooming the preview never lies about the imported
// scale.
//
// Web Mercator metres-per-pixel: 156543.0339 · cos(lat) / 2^zoom,
// then divided by mapScale (sub-zoom interpolation between tile reloads).
func (t *TerrainImport) selectionSquare(sw, sh float32) (x0, y0, x1, y1 float32) {
	cx := sw / 2
	cy := 32 + (sh-32)/2
	half := t.selectionHalfPx()
	return cx - half, cy - half, cx + half, cy + half
}

// selectionHalfPx returns half the side length of the selection square
// in screen pixels for the current grid size + preview zoom.
func (t *TerrainImport) selectionHalfPx() float32 {
	cosLat := math.Cos(t.mapCenter[0] * math.Pi / 180)
	if cosLat < 0.05 {
		cosLat = 0.05 // clamp near poles so we don't blow up
	}
	mppNative := 156543.0339 * cosLat / math.Pow(2, float64(t.mapZoom))
	mpp := mppNative / float64(t.mapScale)
	if mpp <= 0 {
		mpp = 1
	}
	groundMetres := float64(t.gridSize) * importMetersPerCell
	return float32(groundMetres / mpp / 2)
}

// ── OpenStreetMap overlay ─────────────────────────────────────────────────────

var (
	osmLiftColor = mgl32.Vec4{0.95, 0.2, 0.15, 1}
	osmAreaColor = mgl32.Vec4{1, 1, 1, 0.55}
	osmDarkHalo  = mgl32.Vec4{0, 0, 0, 0.6}
	osmLightHalo = mgl32.Vec4{1, 1, 1, 0.75}
)

// osmRunStyle colours runs the North American way, since that's what
// piste:difficulty mostly encodes on US maps; double blacks draw wider.
func osmRunStyle(difficulty string) (color, halo mgl32.Vec4, thick float32) {
	switch difficulty {
	case "novice", "easy":
		return mgl32.Vec4{0.2, 0.85, 0.3, 1}, osmDarkHalo, 2.5
	case "intermediate":
		return mgl32.Vec4{0.25, 0.55, 1, 1}, osmDarkHalo, 2.5
	case "advanced":
		return mgl32.Vec4{0.05, 0.05, 0.05, 1}, osmLightHalo, 2.5
	case "expert":
		return mgl32.Vec4{0.05, 0.05, 0.05, 1}, osmLightHalo, 4
	case "freeride", "extreme":
		return mgl32.Vec4{1, 0.55, 0.1, 1}, osmDarkHalo, 2.5
	}
	return mgl32.Vec4{0.8, 0.8, 0.8, 1}, osmDarkHalo, 2
}

func (d *tiMapDrawable) drawOSM(r *render.Renderer, sw, sh float32) {
	t := d.t
	p := t.projector(sw, sh)
	path := func(pts []geo.LatLon, thick float32, c mgl32.Vec4) {
		px, py := p.at(pts[0])
		for _, ll := range pts[1:] {
			x, y := p.at(ll)
			if x0, y0, x1, y1, ok := clipSegment(px, py, x, y, 0, 32, sw, sh); ok {
				r.DrawColorLine(x0, y0, x1, y1, thick, c)
			}
			px, py = x, y
		}
	}

	for _, a := range t.osm.Areas {
		for _, pts := range a.Paths {
			path(pts, 1.5, osmAreaColor)
		}
	}
	// Halos in their own pass so one run's halo never covers another's line.
	for _, run := range t.osm.Runs {
		_, halo, thick := osmRunStyle(run.Difficulty)
		if run.Area {
			thick = 1.5
		}
		path(run.Path, thick+2, halo)
	}
	for _, run := range t.osm.Runs {
		c, _, thick := osmRunStyle(run.Difficulty)
		if run.Area {
			thick = 1.5
		}
		path(run.Path, thick, c)
	}
	for _, l := range t.osm.Lifts {
		path(l.Path, 5, osmDarkHalo)
	}
	for _, l := range t.osm.Lifts {
		path(l.Path, 3, osmLiftColor)
		for _, end := range []geo.LatLon{l.Path[0], l.Path[len(l.Path)-1]} {
			x, y := p.at(end)
			if y > 32 {
				r.DrawColorDisc(x, y, 4, osmDarkHalo)
				r.DrawColorDisc(x, y, 3, osmLightHalo)
			}
		}
	}

	if r.Font == nil {
		return
	}
	for _, l := range t.osm.Lifts {
		if l.Name == "" {
			continue
		}
		x0, y0 := p.at(l.Path[0])
		x1, y1 := p.at(l.Path[len(l.Path)-1])
		tw := r.Font.TextWidth(l.Name)
		if float32(math.Hypot(float64(x1-x0), float64(y1-y0))) < tw+24 {
			continue
		}
		cx, cy := (x0+x1)/2, (y0+y1)/2
		lx, ly := cx-tw/2, cy-float32(render.GlyphH)/2
		if ly < 34 || ly > sh || lx > sw || lx+tw < 0 {
			continue
		}
		r.DrawColorRect(lx-3, ly-2, tw+6, float32(render.GlyphH)+4, mgl32.Vec4{0, 0, 0, 0.6})
		r.Font.DrawText(r, l.Name, lx, ly, mgl32.Vec4{1, 1, 1, 1})
	}
}

// drawOSMStatus counts what OpenStreetMap has inside the selection
// square, and credits it as the ODbL requires.
func (d *tiMapDrawable) drawOSMStatus(r *render.Renderer, sw, sh float32) {
	t := d.t
	grey := mgl32.Vec4{0.85, 0.85, 0.85, 1}
	panel := mgl32.Vec4{0, 0, 0, 0.6}
	line := func(s string, y float32) {
		r.DrawColorRect(6, y-3, r.Font.TextWidth(s)+8, float32(render.GlyphH)+6, panel)
		r.Font.DrawText(r, s, 10, y, grey)
	}

	y := float32(42)
	switch {
	case t.osmHidden:
		line("Lifts and runs hidden (O to show)", y)
		y += 24
	case len(t.osm.Lifts)+len(t.osm.Runs) > 0:
		p := t.projector(sw, sh)
		qx0, qy0, qx1, qy1 := t.selectionSquare(sw, sh)
		inSquare := func(pts []geo.LatLon) bool {
			for _, ll := range pts {
				if x, y := p.at(ll); x >= qx0 && x <= qx1 && y >= qy0 && y <= qy1 {
					return true
				}
			}
			return false
		}
		lifts, runs := 0, 0
		for _, l := range t.osm.Lifts {
			if inSquare(l.Path) {
				lifts++
			}
		}
		for _, run := range t.osm.Runs {
			if inSquare(run.Path) {
				runs++
			}
		}
		line(fmt.Sprintf("In the square: %d lifts, %d runs", lifts, runs), y)
		y += 24
	}
	if t.osmJob != nil {
		line("Loading lifts and runs from OpenStreetMap...", y)
		y += 24
	}
	if t.osmNote != "" {
		line(t.osmNote, y)
	}

	credit := "Lifts & runs (c) OpenStreetMap contributors"
	cw := r.Font.TextWidth(credit)
	r.DrawColorRect(sw-cw-14, 39, cw+8, float32(render.GlyphH)+6, panel)
	r.Font.DrawText(r, credit, sw-cw-10, 42, grey)
}

// clipSegment clips a segment to the rect (Liang–Barsky); ok is false
// when nothing of it is inside.
func clipSegment(x0, y0, x1, y1, minX, minY, maxX, maxY float32) (float32, float32, float32, float32, bool) {
	t0, t1 := float32(0), float32(1)
	dx, dy := x1-x0, y1-y0
	for _, e := range [4][2]float32{{-dx, x0 - minX}, {dx, maxX - x0}, {-dy, y0 - minY}, {dy, maxY - y0}} {
		p, q := e[0], e[1]
		if p == 0 {
			if q < 0 {
				return 0, 0, 0, 0, false
			}
			continue
		}
		r := q / p
		if p < 0 {
			if r > t1 {
				return 0, 0, 0, 0, false
			}
			if r > t0 {
				t0 = r
			}
		} else {
			if r < t0 {
				return 0, 0, 0, 0, false
			}
			if r < t1 {
				t1 = r
			}
		}
	}
	return x0 + t0*dx, y0 + t0*dy, x0 + t1*dx, y0 + t1*dy, true
}
