package scene

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/geo"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
)

type tisState int

const (
	tisSearch    tisState = iota
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
	elevResult    [][]float32
	progress      float32
	err           error
	done          bool
	cancel        context.CancelFunc
}

func (j *tiJob) setProgress(p float32) { j.mu.Lock(); j.progress = p; j.mu.Unlock() }

func (j *tiJob) finish(err error) {
	j.mu.Lock()
	j.err = err
	j.done = true
	j.mu.Unlock()
}

func (j *tiJob) snapshot() (progress float32, sr []geo.SearchResult, mr *geo.PreviewResult, elev [][]float32, err error, done bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.progress, j.searchResults, j.mapResult, j.elevResult, j.err, j.done
}

// TerrainImport is a full-screen scene for searching, previewing, and importing
// real-world elevation data into the scenario editor terrain.
type TerrainImport struct {
	app      *engine.App
	onImport func([][]float32)

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
}

// NewTerrainImport creates the scene.
// initialGridSize is the starting destination grid side length; the
// player resizes the imported area inside the map view by dragging the
// selection-square corners. onImport is called with the resampled
// elevation grid on success.
func NewTerrainImport(initialGridSize int, onImport func([][]float32)) *TerrainImport {
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

func (t *TerrainImport) Init(app *engine.App) error {
	t.app = app
	t.menuBar = ui.NewMenuBar(0, 32)
	t.menuBar.AddButton("Import Terrain", func() {}) // title label
	t.menuBar.AddButton("Cancel", func() { app.PopScene() })
	return nil
}

func (t *TerrainImport) Update(dt float64) {
	inp := t.app.Input
	r := t.app.Renderer

	t.menuBar.HandleInput(inp, float32(r.ScreenWidth()), float32(r.ScreenHeight()))

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
		if err != nil {
			t.errMsg = err.Error()
			t.state = tisSearch
		} else {
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
		_, _, _, elev, err, done := t.job.snapshot()
		if !done {
			break
		}
		t.job = nil
		if err != nil {
			t.errMsg = err.Error()
			t.state = tisMap
		} else {
			t.onImport(elev)
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
	latSpan := t.mapBounds[1] - t.mapBounds[0]
	lonSpan := t.mapBounds[3] - t.mapBounds[2]
	drawW := float64(t.mapTexW * t.mapScale)
	drawH := float64(t.mapTexH * t.mapScale)
	fx := 0.5 - float64(t.panAccum[0])/drawW
	fy := 0.5 - float64(t.panAccum[1])/drawH
	t.mapCenter[1] = t.mapBounds[2] + fx*lonSpan
	t.mapCenter[0] = t.mapBounds[1] - fy*latSpan
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

	// Match the exact draw position used in Render.
	drawW := t.mapTexW * t.mapScale
	drawH := t.mapTexH * t.mapScale
	drawX := sw/2 - drawW/2 + t.panAccum[0]
	drawY := 32 + (sh-32)/2 - drawH/2 + t.panAccum[1]

	fx0 := float64(sx0-drawX) / float64(drawW)
	fx1 := float64(sx1-drawX) / float64(drawW)
	fy0 := float64(sy0-drawY) / float64(drawH)
	fy1 := float64(sy1-drawY) / float64(drawH)

	latSpan := t.mapBounds[1] - t.mapBounds[0]
	lonSpan := t.mapBounds[3] - t.mapBounds[2]

	fetchMinLat := t.mapBounds[1] - fy1*latSpan
	fetchMaxLat := t.mapBounds[1] - fy0*latSpan
	fetchMinLon := t.mapBounds[2] + fx0*lonSpan
	fetchMaxLon := t.mapBounds[2] + fx1*lonSpan

	// cols/rows are ignored by the tile fetcher but kept for the function signature.
	cols, rows := t.gridSize, t.gridSize

	ctx, cancel := context.WithCancel(context.Background())
	j := &tiJob{cancel: cancel}
	if t.job != nil {
		t.job.cancel()
	}
	t.job = j

	go func() {
		grid, err := geo.FetchGrid(ctx, fetchMinLat, fetchMaxLat, fetchMinLon, fetchMaxLon, cols, rows, j.setProgress)
		if err != nil {
			j.finish(err)
			return
		}
		resampled := geo.ResampleToGrid(grid, t.gridSize, t.gridSize)
		j.mu.Lock()
		j.elevResult = resampled
		j.mu.Unlock()
		j.finish(nil)
	}()
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
			r.Font.DrawText(r, tiTruncate("Error: "+t.errMsg, 64), sw/2-200, sh/2-80, red)
		}
		r.Font.DrawText(r, "Search for a location:", sw/2-120, sh/2-50, grey)
		fx, fy, fw, fh := sw/2-200, sh/2-26, float32(400), float32(32)
		r.DrawColorRect(fx, fy, fw, fh, mgl32.Vec4{0.05, 0.05, 0.12, 1})
		r.DrawColorRect(fx, fy, fw, 1, mgl32.Vec4{0.5, 0.5, 0.9, 1})
		r.DrawColorRect(fx, fy+fh-1, fw, 1, mgl32.Vec4{0.5, 0.5, 0.9, 1})
		r.DrawColorRect(fx, fy, 1, fh, mgl32.Vec4{0.5, 0.5, 0.9, 1})
		r.DrawColorRect(fx+fw-1, fy, 1, fh, mgl32.Vec4{0.5, 0.5, 0.9, 1})
		r.Font.DrawText(r, t.searchBuf+"_", fx+6, fy+6, white)
		r.Font.DrawText(r, "Press Enter to search", sw/2-100, sh/2+14, grey)

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
			r.Font.DrawText(r, tiTruncate(res.DisplayName, 62), listX+8, rowY+12, white)
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
		r.Font.DrawText(r, "Left-drag: pan   Scroll: zoom   Drag a corner of the square to resize", 10, sh-22, grey)
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
		progress, _, _, _, _, _ := t.job.snapshot()
		pct := int(progress * 100)
		r.Font.DrawText(r, fmt.Sprintf("Fetching elevation... %d%%", pct), sw/2-120, sh/2-20, white)
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
