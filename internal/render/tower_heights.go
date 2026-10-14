package render

import (
	"math"

	"mountain-mogul/internal/world"

	"github.com/go-gl/mathgl/mgl32"
)

// Towers of every height from one model each. A lift's towers are as tall
// as its laid-out cable needs (world.Lift.Towers), so each tower model is
// stretched to the height asked for, a whole metre at a time, and kept as
// its own static batch: its head (the crossarm, sheaves or saddles over
// the cable) moves up or down whole, and the shaft under it (the pole and
// ladder, or the lattice and its bracing) stretches to meet it, spreading
// wider at the foot as it gets taller.

// towerModel is a tower mesh's model and the cable height it's built for.
type towerModel struct {
	name  string
	lineH float32
}

var towerModels = map[uint32]towerModel{
	MeshTower:        {"tower", world.TowerHeight},
	MeshTowerWide:    {"tower_wide", world.TowerHeight},
	MeshFunitelTower: {"funitel_tower", world.FunitelLineHeight},
	MeshTramTower:    {"tram_tower", world.TramLineHeight},
}

// towerHeadDepth is how far under the cable a tower's head reaches: all
// of the model above lineH less this moves whole.
const towerHeadDepth = 3

// towerHeightMesh is the mesh ID of tower mesh base stretched to carry
// its cable h metres up, clear of every fixed mesh ID.
func towerHeightMesh(base uint32, h int) uint32 {
	return 1<<16 | base<<8 | uint32(h)
}

// towerBatch is the static batch of tower mesh base at height (metres,
// rounded to h), its mesh built on first use. Mesh base itself for a
// tower without heights (the T-bar's) or at its model's own height.
func (r *Renderer) towerBatch(base uint32, height float32) (id uint32, b *Batch, h float32) {
	tm, ok := towerModels[base]
	if !ok {
		return base, r.staticBatches[base], height
	}
	if hm := int(math.Round(float64(height))); hm == int(math.Round(float64(tm.lineH))) {
		return base, r.staticBatches[base], tm.lineH
	}
	return r.towerHeightBatch(base, tm, int(math.Round(float64(height))))
}

func (r *Renderer) towerHeightBatch(base uint32, tm towerModel, h int) (uint32, *Batch, float32) {
	h = min(max(h, 4), 255)
	id := towerHeightMesh(base, h)
	if b, ok := r.staticBatches[id]; ok {
		return id, b, float32(h)
	}
	if _, ok := r.staticBatches[base]; !ok {
		return id, nil, 0 // not in this edition
	}
	mesh, tex, err := loadOBJFile(r.assetDir+"/models/"+tm.name+".obj", towerStretch(tm.lineH, float32(h)))
	if err != nil {
		mesh, tex = fallbackMesh()
	}
	b := NewStaticBatch(mesh, tex)
	r.staticBatches[id] = b
	return id, b, float32(h)
}

// towerStretch moves a tower model built for a cable lineH up to one
// carrying it h up: the head (above lineH - towerHeadDepth) up or down
// whole, the shaft under it stretched to meet it, and the shaft's foot
// spread by the square root of the stretch (the footing too).
func towerStretch(lineH, h float32) func([3]float32) [3]float32 {
	split := lineH - towerHeadDepth
	lift := h - lineH
	s := (split + lift) / split
	spread := float32(math.Sqrt(float64(h / lineH)))
	spread = min(max(spread, 0.85), 1.6)
	return func(p [3]float32) [3]float32 {
		y := p[1]
		switch {
		case y > split:
			return [3]float32{p[0], y + lift, p[2]}
		case y <= 0:
			return [3]float32{p[0] * spread, y, p[2] * spread}
		}
		w := 1 + (spread-1)*(1-y/split)
		return [3]float32{p[0] * w, y * s, p[2] * w}
	}
}

// towerInstance is one tower to draw: its mesh (at its height) and where.
type towerInstance struct {
	mesh  uint32
	batch *Batch
	m     mgl32.Mat4
}

// towerInstances is the lift's towers, each on the mesh of its height,
// turned so its model-space +X (the cable axis convention from
// models-src/tower.py) runs along the cable.
func (r *Renderer) towerInstances(lift *world.Lift, t *world.Terrain) []towerInstance {
	towers := lift.Towers(t)
	if len(towers) == 0 {
		return nil
	}
	d := lift.Top.Sub(lift.Base)
	// HomogRotate3DY(θ) takes (1,0,0) → (cos θ, 0, -sin θ); we want that
	// to equal the cable direction (dx, 0, dz), so θ = atan2(-dz, dx).
	rot := mgl32.HomogRotate3DY(float32(math.Atan2(-float64(d[1]), float64(d[0]))))
	base := TowerMesh(lift.Type)
	out := make([]towerInstance, 0, len(towers))
	for _, tw := range towers {
		id, b, h := r.towerBatch(base, tw.Height)
		if b == nil {
			continue
		}
		// Its head on the cable, where the profile put it (the tower's
		// height over the bare ground, as laid out), so a mesh rounded
		// to the metre sinks or rises a little at the foot instead.
		p := tw.XZ
		y := t.InterpolatedGroundElevationAt(p[0], p[1]) + tw.Height - h
		if lift.Type.Hangers() {
			y = VisualElevationAt(t, p[0], p[1])
		}
		m := mgl32.Translate3D(p[0], y, p[1]).Mul4(rot)
		out = append(out, towerInstance{mesh: id, batch: b, m: m})
	}
	return out
}
