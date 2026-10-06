package scene

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/geo"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// layersPanel is the editor's Terrain layers panel: the import's base
// ground, which can be reopened to change the square, then each terrain
// layer with a checkbox: the ground layers (geo.Layers), then the world
// layers (worldLayers), each with a strength slider. Switching a ground
// layer, or releasing its slider, rebuilds the ground from the base in
// the background, then reruns the world layers on it.
type layersPanel struct {
	open      bool
	stack     *geo.LayerStack // runs on the world's TerrainBase; nil until needed
	job       *layerJob
	toggleBtn *ui.Button // the menu bar's Layers button
	changeBtn *ui.Button

	// queued lists world layers to run or clear, by ID. They wait a
	// frame so the panel can show them running first.
	queued     []string
	queuedWait bool

	// dragging says a strength slider is held: row dragRow, at dragVal.
	// The strength changes when it's released.
	dragging bool
	dragRow  int
	dragVal  float32

	x, y, w, h float32 // from the last layout
	rowsY      float32 // top of the first layer row
}

// layerJob is one background rebuild of the ground.
type layerJob struct {
	stack *geo.LayerStack
	set   []geo.Setting // what it ran with

	mu      sync.Mutex
	stage   string
	heights []float32
	done    bool
}

const (
	layersPanelW = float32(470)
	layersRowH   = float32(30)
	layersPad    = float32(10)

	// Each row's strength slider: its track's left edge from the
	// panel's, and its width.
	layersSliderX = float32(185)
	layersSliderW = float32(130)
)

func (j *layerJob) status() (stage string, done bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.stage, j.done
}

func (p *layersPanel) layout(top float32, base *world.TerrainBase) {
	lineH := float32(render.GlyphH + 6)
	p.x, p.y, p.w = 12, top+8, layersPanelW
	titleH := lineH + 4
	baseH := 2*lineH + layersPad
	p.rowsY = p.y + titleH + baseH
	rows := 0
	if base != nil {
		rows = len(geo.Layers) + len(worldLayers) + 1 // and the OpenStreetMap overlay
	}
	p.h = p.rowsY - p.y + float32(rows)*layersRowH + lineH + layersPad
	if p.changeBtn != nil {
		p.changeBtn.W, p.changeBtn.H = 110, 28
		p.changeBtn.X = p.x + p.w - layersPad - p.changeBtn.W
		p.changeBtn.Y = p.y + titleH + (baseH-p.changeBtn.H)/2
	}
}

// rowLayer is the layer in panel row i: its ID, and its index in
// geo.Layers, or -1 for a world layer.
func rowLayer(i int) (id string, ground int) {
	if i < len(geo.Layers) {
		return geo.Layers[i].ID, i
	}
	return worldLayers[i-len(geo.Layers)].ID, -1
}

// rowHasSlider reports whether row i shows a strength slider: every
// layer but a ground layer that does nothing here. The OpenStreetMap
// overlay's row, last, has none.
func rowHasSlider(i int, base *world.TerrainBase) bool {
	if i >= len(geo.Layers)+len(worldLayers) {
		return false
	}
	return i >= len(geo.Layers) || geo.Applies(i, base)
}

// sliderValue is the strength at screen x on a row's slider.
func (p *layersPanel) sliderValue(mx float32) float32 {
	v := (mx - p.x - layersSliderX) / layersSliderW
	return float32(math.Round(float64(min(max(v, 0), 1))*100)) / 100
}

func (p *layersPanel) contains(mx, my float32) bool {
	return p.open && mx >= p.x && mx <= p.x+p.w && my >= p.y && my <= p.y+p.h
}

// toggleLayersPanel opens or closes the panel. Opening it drops the
// active tool, whose sliders share the left edge.
func (e *Editor) toggleLayersPanel() {
	p := &e.layers
	p.open = !p.open
	if p.open && e.activeTool != toolNone {
		e.setTool(e.activeTool)
	}
	p.toggleBtn.SetActive(p.open)
}

// ShowLayers opens the Layers panel with exactly the layers in off
// switched off, rebuilding the ground if that changes it, without
// asking first. For screenshots.
func (e *Editor) ShowLayers(off []string) {
	if !e.layers.open {
		e.toggleLayersPanel()
	}
	base := e.world.TerrainBase
	if base == nil || slices.Equal(base.LayersOff, off) {
		return
	}
	ground := !slices.Equal(groundOff(base), groundOff(&world.TerrainBase{LayersOff: off}))
	for _, l := range worldLayers {
		if base.LayerOn(l.ID) != !slices.Contains(off, l.ID) {
			base.SetLayer(l.ID, !base.LayerOn(l.ID))
			if !ground {
				e.queueWorldLayer(l.ID)
			}
		}
	}
	base.LayersOff = off
	if ground {
		e.startLayerRebuild()
	}
}

// LayersSettled reports that no layer is waiting to run.
func (e *Editor) LayersSettled() bool { return e.layers.job == nil && len(e.layers.queued) == 0 }

// groundOff is base's switched-off ground layers.
func groundOff(base *world.TerrainBase) []string {
	var off []string
	for _, l := range geo.Layers {
		if !base.LayerOn(l.ID) {
			off = append(off, l.ID)
		}
	}
	return off
}

// handleLayersInput runs the panel's clicks and slider drags, consuming
// them.
func (e *Editor) handleLayersInput(inp *engine.Input, top float32) {
	e.pollLayerJob()
	e.runQueuedWorldLayers()
	p := &e.layers
	base := e.world.TerrainBase
	if !p.open || base == nil {
		p.dragging = false
	}
	if !p.open {
		return
	}
	p.layout(top, base)
	mx, my := inp.MousePos[0], inp.MousePos[1]
	if p.dragging {
		p.dragVal = p.sliderValue(mx)
		inp.LeftClickConsumed = true
		if !inp.LeftHeld {
			p.dragging = false
			e.requestStrength(p.dragRow, p.dragVal)
		}
		return
	}
	p.changeBtn.SetHovered(p.changeBtn.Contains(mx, my))
	if !inp.LeftClick || inp.LeftClickConsumed || !p.contains(mx, my) {
		return
	}
	inp.LeftClickConsumed = true
	if p.changeBtn.Contains(mx, my) {
		p.changeBtn.Click()
		return
	}
	if base == nil || my < p.rowsY {
		return
	}
	i := int((my - p.rowsY) / layersRowH)
	switch {
	case i == len(geo.Layers)+len(worldLayers):
		e.toggleOSMOverlay()
		return
	case i > len(geo.Layers)+len(worldLayers):
		return
	}
	const grab = 8 // px either side of the track that still grab it
	sx := p.x + layersSliderX
	if mx >= sx-grab && rowHasSlider(i, base) {
		if mx <= sx+layersSliderW+grab {
			p.dragging, p.dragRow, p.dragVal = true, i, p.sliderValue(mx)
		}
		return
	}
	switch id, ground := rowLayer(i); {
	case ground >= 0:
		e.requestLayerToggle(ground)
	default:
		base.SetLayer(id, !base.LayerOn(id))
		e.markDirty()
		e.queueWorldLayer(id)
	}
}

// requestStrength sets row i's layer to strength v and reruns it when
// it's on, first asking when that clears what's been built.
func (e *Editor) requestStrength(i int, v float32) {
	base := e.world.TerrainBase
	id, ground := rowLayer(i)
	if base == nil || v == base.Strength(id) {
		return
	}
	set := func() {
		e.confirmPrompt = nil
		base.SetStrength(id, v)
		e.markDirty()
		switch {
		case !base.LayerOn(id):
		case ground >= 0:
			e.startLayerRebuild()
		default:
			e.queueWorldLayer(id)
		}
	}
	if ground >= 0 && base.LayerOn(id) && geo.Layers[ground].MovesGround && worldHasBuilt(e.world) {
		e.confirmPrompt = newConfirmPrompt(
			"Changing "+geo.Layers[ground].Name+"'s strength reshapes the ground and removes every lift, building, and road. Trails and parcels stay, and the Auto layers that are on grow again.",
			"Continue", set, func() { e.confirmPrompt = nil })
		return
	}
	set()
}

// queueWorldLayer runs world layer id, or clears it when it's off, from
// the next frame but one.
func (e *Editor) queueWorldLayer(id string) {
	p := &e.layers
	if !slices.Contains(p.queued, id) {
		p.queued = append(p.queued, id)
	}
	p.queuedWait = true
}

func (e *Editor) runQueuedWorldLayers() {
	p := &e.layers
	if len(p.queued) == 0 || p.job != nil {
		return
	}
	if p.queuedWait {
		p.queuedWait = false
		return
	}
	w := e.world
	for _, l := range worldLayers {
		if !slices.Contains(p.queued, l.ID) || w.TerrainBase == nil {
			continue
		}
		if !w.TerrainBase.LayerOn(l.ID) {
			l.clear(w)
			continue
		}
		l.run(w, &e.layerCache)
	}
	p.queued = p.queued[:0]
	restampClearings(w)
	e.refreshWorldLayers()
	e.osm.built = time.Time{}
}

// refreshWorldLayers sends what the world layers changed to the renderer.
func (e *Editor) refreshWorldLayers() {
	if e.app == nil || e.app.Renderer == nil {
		return
	}
	r := e.app.Renderer
	r.FlushTerrainVerts(e.world.Terrain)
	r.BuildGroomTex(e.world.Terrain)
	r.RebuildStaticBatch(e.world)
}

// requestLayerToggle switches layer i, first asking when that clears
// what's been built.
func (e *Editor) requestLayerToggle(i int) {
	base := e.world.TerrainBase
	l := geo.Layers[i]
	if !geo.Applies(i, base) {
		e.setToast(l.Name + " does nothing here: " + layerSkipReason(i, base))
		return
	}
	toggle := func() {
		e.confirmPrompt = nil
		base.SetLayer(l.ID, !base.LayerOn(l.ID))
		e.markDirty()
		e.startLayerRebuild()
	}
	if l.MovesGround && worldHasBuilt(e.world) {
		verb := "Switching off "
		if !base.LayerOn(l.ID) {
			verb = "Switching on "
		}
		e.confirmPrompt = newConfirmPrompt(
			verb+l.Name+" reshapes the ground and removes every lift, building, and road. Trails and parcels stay, and the Auto layers that are on grow again.",
			"Continue", toggle, func() { e.confirmPrompt = nil })
		return
	}
	toggle()
}

// requestReimport reopens the import on the base's square (or a fresh
// import when there's no base), first asking when there's anything on
// the map to lose.
func (e *Editor) requestReimport() {
	open := func() {
		e.confirmPrompt = nil
		done := func(imp ImportedTerrain) { e.applyImportedTerrain(imp, e.app.Renderer) }
		if base := e.world.TerrainBase; base != nil {
			e.app.PushScene(NewTerrainReimport(e.world.Terrain.Width, base, done))
		} else {
			e.app.PushScene(NewTerrainImport(e.world.Terrain.Width, done))
		}
	}
	w := e.world
	if worldHasBuilt(w) || len(w.Trails) > 0 || len(w.Parcels) > 0 || w.Terrain.TotalTrees() > 0 {
		e.confirmPrompt = newConfirmPrompt(
			"A new import replaces the whole map: everything built, trails, trees, snow, and parcels. Cancelling the import keeps this one.",
			"Continue", open, func() { e.confirmPrompt = nil })
		return
	}
	open()
}

func worldHasBuilt(w *world.World) bool {
	return len(w.Buildings) > 0 || len(w.Lifts) > 0 || len(w.RoadNodes) > 0
}

func layerSkipReason(i int, base *world.TerrainBase) string {
	switch {
	case geo.Layers[i].DetailOnly && !base.Detail:
		return "needs lidar"
	case base.RoadNote != "":
		return base.RoadNote
	}
	return "no roads"
}

// startLayerRebuild rebuilds the ground from the base with the current
// layer switches and strengths, unless a rebuild is already running;
// that one starts another when it finishes if they've changed since.
func (e *Editor) startLayerRebuild() {
	p := &e.layers
	base := e.world.TerrainBase
	if base == nil || p.job != nil {
		return
	}
	if p.stack == nil || p.stack.Base() != base {
		p.stack = geo.NewLayerStack(base)
	}
	j := &layerJob{stack: p.stack, set: geo.Settings(base), stage: "Starting"}
	p.job = j
	go func() {
		h := j.stack.Run(j.set, func(name string) {
			j.mu.Lock()
			j.stage = name
			j.mu.Unlock()
		})
		j.mu.Lock()
		j.heights, j.done = h, true
		j.mu.Unlock()
	}()
}

// pollLayerJob applies a finished rebuild, and starts the next one when
// the switches or strengths changed while it ran.
func (e *Editor) pollLayerJob() {
	p := &e.layers
	j := p.job
	if j == nil {
		return
	}
	if _, done := j.status(); !done {
		return
	}
	p.job = nil
	base := e.world.TerrainBase
	if base == nil || j.stack != p.stack {
		return // the map was replaced while it ran
	}
	e.applyLayerHeights(base, j.heights)
	if !slices.Equal(j.set, geo.Settings(base)) {
		e.startLayerRebuild()
	}
}

// applyLayerHeights sets the ground to heights, clearing what was built
// on the old ground, and reruns the world layers on it.
func (e *Editor) applyLayerHeights(base *world.TerrainBase, heights []float32) {
	w := e.world
	w.ClearBuilt()
	if err := geo.ApplyHeights(w.Terrain, base, heights); err != nil {
		e.setToast("Rebuild failed: " + err.Error())
		return
	}
	e.layerCache = layerCache{}
	runWorldLayers(w, &e.layerCache, nil)
	e.layers.queued = e.layers.queued[:0]
	e.roadEdit.clear()
	e.structureEdit.clear()
	e.parcelBoundaryDirty = true
	e.osm.built = time.Time{}
	r := e.app.Renderer
	r.ResetSceneState()
	r.BuildTerrainMesh(w.Terrain)
	r.BuildSnowSurfaceTex(w.Terrain)
	r.BuildGroomTex(w.Terrain)
	r.RebuildStaticBatch(w)
	r.RebuildRoads(w)
}

// drawLayersPanel draws the panel.
func (e *Editor) drawLayersPanel(r *render.Renderer, top float32) {
	p := &e.layers
	base := e.world.TerrainBase
	p.layout(top, base)
	lineH := float32(render.GlyphH + 6)
	white := mgl32.Vec4{0.92, 0.95, 1, 1}
	dim := mgl32.Vec4{0.6, 0.65, 0.75, 1}
	r.DrawColorRect(p.x, p.y, p.w, p.h, mgl32.Vec4{0.08, 0.10, 0.14, 0.95})
	r.DrawColorRect(p.x, p.y, p.w, lineH+4, mgl32.Vec4{0.15, 0.20, 0.35, 1.0})
	if r.Font == nil {
		return
	}
	f := r.Font
	f.DrawText(r, "Terrain layers", p.x+8, p.y+5, white)

	y := p.y + lineH + 4 + layersPad/2
	if base == nil {
		p.changeBtn.Label = "Import..."
		f.DrawText(r, "Base: none", p.x+layersPad, y+3, white)
		f.DrawText(r, "Import a real place first", p.x+layersPad, y+lineH+3, dim)
	} else {
		p.changeBtn.Label = "Change..."
		t := e.world.Terrain
		km := float32(t.Width-1) * world.CellSize / 1000
		f.DrawText(r, "Base import", p.x+layersPad, y+3, white)
		info := fmt.Sprintf("%.1f km, no lidar", km)
		if base.Detail {
			info = fmt.Sprintf("%.1f km, lidar %.0f%%", km, 100*base.LidarCoverage)
		}
		f.DrawText(r, info, p.x+layersPad, y+lineH+3, dim)
	}
	p.changeBtn.Draw(r)

	stage := ""
	if p.job != nil {
		stage, _ = p.job.status()
	}
	row := func(i int, name string, on, applies bool, note string) {
		ry := p.rowsY + float32(i)*layersRowH
		col := white
		if !applies {
			col = dim
		}
		bx, by := p.x+layersPad, ry+(layersRowH-16)/2
		r.DrawColorRectOutline(bx, by, 16, 16, col)
		if on {
			r.DrawColorRect(bx+4, by+4, 8, 8, col)
		}
		ty := ry + (layersRowH-float32(render.GlyphH))/2
		f.DrawText(r, name, bx+26, ty, col)
		sx := p.x + layersSliderX
		if !rowHasSlider(i, base) {
			f.DrawText(r, note, sx, ty, dim)
			return
		}
		id, _ := rowLayer(i)
		v := base.Strength(id)
		if p.dragging && p.dragRow == i {
			v = p.dragVal
		}
		fill := mgl32.Vec4{0.25, 0.55, 0.35, 0.95}
		if !on {
			fill = mgl32.Vec4{0.3, 0.34, 0.4, 0.95}
		}
		cy := ry + layersRowH/2
		r.DrawColorRect(sx, cy-2, layersSliderW, 4, mgl32.Vec4{0.18, 0.22, 0.30, 1})
		r.DrawColorRect(sx, cy-2, layersSliderW*v, 4, fill)
		// A tick at the default strength, the standard pass.
		r.DrawColorRect(sx+layersSliderW*world.DefaultLayerStrength-1, cy-5, 2, 10, dim)
		r.DrawColorRect(sx+layersSliderW*v-4, cy-8, 8, 16, col)
		f.DrawText(r, fmt.Sprintf("%.0f%%", 100*v), sx+layersSliderW+12, ty, col)
		f.DrawText(r, note, p.x+p.w-layersPad-f.TextWidth(note), ty, dim)
	}
	if base != nil {
		for i, l := range geo.Layers {
			applies := geo.Applies(i, base)
			note := ""
			switch {
			case !applies:
				note = layerSkipNote(i, base)
			case p.job != nil && stage == l.Name:
				note = "running"
			}
			row(i, l.Name, base.LayerOn(l.ID), applies, note)
		}
		for i, l := range worldLayers {
			on := base.LayerOn(l.ID)
			note := ""
			switch {
			case p.job != nil && on, slices.Contains(p.queued, l.ID):
				note = "running"
			case on:
				note = l.note(e.world)
			}
			row(len(geo.Layers)+i, l.Name, on, true, note)
		}
		row(len(geo.Layers)+len(worldLayers), "OpenStreetMap", e.osm.shown, true, osmCounts(base))
	}
	status := ""
	switch {
	case p.job != nil:
		status = "Rebuilding the ground..."
	case len(p.queued) > 0:
		status = "Running..."
	}
	f.DrawText(r, status, p.x+layersPad, p.y+p.h-lineH-layersPad/2+3, dim)
}

// layerSkipNote is layerSkipReason cut to fit beside a row's name.
func layerSkipNote(i int, base *world.TerrainBase) string {
	s := layerSkipReason(i, base)
	if strings.HasPrefix(s, "couldn't fetch") {
		return "roads not fetched"
	}
	return s
}
