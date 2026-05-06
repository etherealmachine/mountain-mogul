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
	app         *engine.App
	onImport    func([][]float32)
	terrainCols int
	terrainRows int

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

	job     *tiJob
	menuBar *ui.MenuBar
}

// NewTerrainImport creates the scene.
// terrainCols/Rows are the destination grid size for the elevation fetch.
// onImport is called with the resampled elevation grid on success.
func NewTerrainImport(terrainCols, terrainRows int, onImport func([][]float32)) *TerrainImport {
	return &TerrainImport{
		onImport:    onImport,
		terrainCols: terrainCols,
		terrainRows: terrainRows,
		state:       tisSearch,
		mapZoom:     12,
		mapScale:    1.0,
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

	t.menuBar.HandleInput(inp, float32(r.ScreenHeight()))

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

	// ── Left drag: pan ───────────────────────────────────────────────────────
	if inp.LeftClick && inMap && !t.mapLoading {
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
	}

	// ── Import button ────────────────────────────────────────────────────────
	if t.mapTexID != 0 && inp.LeftClick {
		btnX := sw - 140
		btnY := sh - 50
		if mx >= btnX && mx <= btnX+130 && my >= btnY && my <= btnY+36 {
			t.startFetch()
		}
	}
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

	// Convert the fixed selection square to lat/lon bounds.
	// panOffset is 0 here — pan is committed before Import can be clicked.
	sw := float32(t.app.Renderer.ScreenWidth())
	sh := float32(t.app.Renderer.ScreenHeight())
	sx0, sy0, sx1, sy1 := selectionSquare(sw, sh)

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
	cols, rows := t.terrainCols, t.terrainRows

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
		resampled := geo.ResampleToGrid(grid, t.terrainCols, t.terrainRows)
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

	// Fixed selection square — always centered on the map area.
	if t.mapTexID != 0 {
		qx0, qy0, qx1, qy1 := selectionSquare(sw, sh)
		thick := float32(2)
		r.DrawColorRect(qx0, qy0, qx1-qx0, thick, yellow)
		r.DrawColorRect(qx0, qy1-thick, qx1-qx0, thick, yellow)
		r.DrawColorRect(qx0, qy0, thick, qy1-qy0, yellow)
		r.DrawColorRect(qx1-thick, qy0, thick, qy1-qy0, yellow)
	}

	// Hint
	if t.mapTexID != 0 && t.state != tisFetching {
		r.Font.DrawText(r, "Left-drag: pan   Scroll: zoom   Center the square on your target area", 10, sh-22, grey)
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

// selectionSquare returns the screen corners of the fixed import-area square,
// centered in the map area below the menu bar.
func selectionSquare(sw, sh float32) (x0, y0, x1, y1 float32) {
	half := minF(sw, sh-32) * 0.65 / 2
	cx := sw / 2
	cy := 32 + (sh-32)/2
	return cx - half, cy - half, cx + half, cy + half
}
