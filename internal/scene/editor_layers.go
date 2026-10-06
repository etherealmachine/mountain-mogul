package scene

import (
	"fmt"
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
// layers (worldLayers). Switching a ground layer rebuilds the ground from
// the base in the background, then reruns the world layers on it.
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
	worldTook  map[string]time.Duration

	x, y, w, h float32 // from the last layout
	rowsY      float32 // top of the first layer row
}

// layerJob is one background rebuild of the ground.
type layerJob struct {
	stack *geo.LayerStack
	off   []string // the layers it ran with off

	mu      sync.Mutex
	stage   string
	heights []float32
	done    bool
}

const (
	layersPanelW = float32(400)
	layersRowH   = float32(30)
	layersPad    = float32(10)
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
		rows = len(geo.Layers) + len(worldLayers)
	}
	p.h = p.rowsY - p.y + float32(rows)*layersRowH + lineH + layersPad
	if p.changeBtn != nil {
		p.changeBtn.W, p.changeBtn.H = 110, 28
		p.changeBtn.X = p.x + p.w - layersPad - p.changeBtn.W
		p.changeBtn.Y = p.y + titleH + (baseH-p.changeBtn.H)/2
	}
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

// handleLayersInput runs the panel's clicks, consuming them.
func (e *Editor) handleLayersInput(inp *engine.Input, top float32) {
	e.pollLayerJob()
	e.runQueuedWorldLayers()
	p := &e.layers
	if !p.open {
		return
	}
	base := e.world.TerrainBase
	p.layout(top, base)
	mx, my := inp.MousePos[0], inp.MousePos[1]
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
	switch i := int((my - p.rowsY) / layersRowH); {
	case i < len(geo.Layers):
		e.requestLayerToggle(i)
	case i < len(geo.Layers)+len(worldLayers):
		id := worldLayers[i-len(geo.Layers)].ID
		base.SetLayer(id, !base.LayerOn(id))
		e.markDirty()
		e.queueWorldLayer(id)
	}
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
			delete(p.worldTook, l.ID)
			continue
		}
		start := time.Now()
		l.run(w, &e.layerCache)
		if p.worldTook == nil {
			p.worldTook = map[string]time.Duration{}
		}
		p.worldTook[l.ID] = time.Since(start)
	}
	p.queued = p.queued[:0]
	restampClearings(w)
	e.refreshWorldLayers()
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
// layer switches, unless a rebuild is already running; that one starts
// another when it finishes if the switches have changed since.
func (e *Editor) startLayerRebuild() {
	p := &e.layers
	base := e.world.TerrainBase
	if base == nil || p.job != nil {
		return
	}
	if p.stack == nil || p.stack.Base() != base {
		p.stack = geo.NewLayerStack(base)
	}
	j := &layerJob{stack: p.stack, off: groundOff(base), stage: "Starting"}
	p.job = j
	go func() {
		h := j.stack.Run(j.off, func(name string) {
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
// the switches changed while it ran.
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
	if !slices.Equal(j.off, groundOff(base)) {
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
	e.layers.worldTook = runWorldLayers(w, &e.layerCache, nil)
	e.layers.queued = e.layers.queued[:0]
	e.roadEdit.clear()
	e.structureEdit.clear()
	e.parcelBoundaryDirty = true
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
			case p.stack != nil && p.stack.Took[i] > 0:
				note = formatTook(p.stack.Took[i])
			}
			row(i, l.Name, base.LayerOn(l.ID), applies, note)
		}
		for i, l := range worldLayers {
			on := base.LayerOn(l.ID)
			note := ""
			switch {
			case p.job != nil && on, slices.Contains(p.queued, l.ID):
				note = "running"
			case !on:
			case l.note(e.world) != "":
				note = l.note(e.world)
			case p.worldTook[l.ID] > 0:
				note = formatTook(p.worldTook[l.ID])
			}
			row(len(geo.Layers)+i, l.Name, on, true, note)
		}
	}
	status := "Uncheck a layer to see without it"
	switch {
	case p.job != nil:
		status = "Rebuilding the ground..."
	case len(p.queued) > 0:
		status = "Running..."
	case base == nil:
		status = ""
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

func formatTook(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0f ms", float64(d)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%.1f s", d.Seconds())
}
