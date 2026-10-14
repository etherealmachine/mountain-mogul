package world

import "math"

// Lake is a lake or pond on the map, levelled by the Lakes terrain layer,
// and its ice through the season. Its cells are marked in Terrain.LakeOf
// with their depth in Terrain.LakeDepth, and its surface in the material
// map shows open water, thin ice, or ice spot by spot.
//
// Ice is tracked as two running totals for the whole lake. Frost is the
// cold felt since autumn (°C·days below freezing, less any warm days
// before ice formed); a spot freezes once Frost passes FreezeFrostPerM
// times its depth, so the shallows skin over first and the ice creeps out
// over weeks. Thaw is the warmth felt since then, which melts the ice,
// fastest in the sun-warmed shallows.
type Lake struct {
	Name     string
	Altitude float32 // metres above sea level of the surface
	AreaHa   float32 // the whole lake, including any part off the map
	MaxDepth float32 // metres, its deepest cell
	Frost    float32 // °C·days
	Thaw     float32 // °C·days
	// Cover is the editor's setting (WaterLake.Cover): Auto runs the ice
	// as above; an Open lake never freezes, a Covered one is frozen (at
	// least CoveredIce thick) all winter.
	Cover WaterCover
}

// CoveredIce is the least ice on a lake set to stay Covered, metres:
// enough to hold snow everywhere.
const CoveredIce = 0.3

const (
	// FreezeFrostPerM is the frost, °C·days, a metre of water depth needs
	// before its surface freezes: deeper water has more heat to lose.
	FreezeFrostPerM = 2.5
	// iceGrowth is m of ice per √(°C·day) of frost once frozen (Stefan's
	// law), slowed to 0.6× by the snow that usually lies on lake ice.
	iceGrowth = 0.027 * 0.6
	// iceMelt is m of ice melted per °C·day of thaw in deep water; the
	// shallows melt up to three times as fast.
	iceMelt = 0.005
	// ThinIce is the ice under which a spot shows as thin, dark ice that
	// holds little snow, metres.
	ThinIce = 0.05
)

// LakeDepthAt is the water depth, metres, at world (wx, wz), bilinear
// between grid points; 0 off the lakes.
func (t *Terrain) LakeDepthAt(wx, wz float32) float32 {
	if t.LakeDepth == nil {
		return 0
	}
	gx, gz := wx/CellSize, wz/CellSize
	x0, z0 := int(math.Floor(float64(gx))), int(math.Floor(float64(gz)))
	fx, fz := gx-float32(x0), gz-float32(z0)
	at := func(x, z int) float32 {
		x, z = min(max(x, 0), t.Width-1), min(max(z, 0), t.Height-1)
		return t.LakeDepth[x*t.Height+z]
	}
	a := at(x0, z0)*(1-fx) + at(x0+1, z0)*fx
	b := at(x0, z0+1)*(1-fx) + at(x0+1, z0+1)*fx
	return a*(1-fz) + b*fz
}

// LakeNear is the lake (1 + its index in World.Lakes) of material sample
// (i, j) on a lake's water: its nearest grid point's, else any of the
// grid points round it (a sample on the rim, where the nearest point is
// shore); 0 for none.
func (t *Terrain) LakeNear(i, j int) uint8 {
	if t.LakeOf == nil {
		return 0
	}
	at := func(x, z int) uint8 {
		x, z = min(max(x, 0), t.Width-1), min(max(z, 0), t.Height-1)
		return t.LakeOf[x*t.Height+z]
	}
	if id := at((i+DetailPerCell/2)/DetailPerCell, (j+DetailPerCell/2)/DetailPerCell); id != 0 {
		return id
	}
	x, z := i/DetailPerCell, j/DetailPerCell
	for _, d := range [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		if id := at(x+d[0], z+d[1]); id != 0 {
			return id
		}
	}
	return 0
}

// IceAt is the ice thickness, metres, where the lake is depth metres deep.
func (l *Lake) IceAt(depth float32) float32 {
	switch l.Cover {
	case WaterOpen:
		return 0
	case WaterCovered:
		return max(l.autoIceAt(depth), CoveredIce)
	}
	return l.autoIceAt(depth)
}

// autoIceAt is IceAt as the season makes it.
func (l *Lake) autoIceAt(depth float32) float32 {
	grown := l.Frost - FreezeFrostPerM*depth
	if grown <= 0 {
		return 0
	}
	h := iceGrowth*float32(math.Sqrt(float64(grown))) - iceMelt*l.Thaw*(1+2/(1+depth))
	return max(h, 0)
}

// Frozen reports whether any of the lake has ice.
func (l *Lake) Frozen() bool {
	for _, d := range []float32{0, 0.5, 1, 2, 4, l.MaxDepth / 2, l.MaxDepth} {
		if l.IceAt(d) > 0 {
			return true
		}
	}
	return false
}

// SurfaceAt is the material the lake shows where it's depth metres deep.
func (l *Lake) SurfaceAt(depth float32) Material {
	switch h := l.IceAt(depth); {
	case h <= 0:
		return MatOpenWater
	case h < ThinIce:
		return MatThinIce
	}
	return MatIce
}

// Step advances the lake by one day with mean air temperature airC at
// its surface.
func (l *Lake) Step(airC float32) {
	switch {
	case airC < 0:
		l.Frost -= airC
	case l.Frozen():
		l.Thaw += airC
	default:
		// No ice anywhere: warm days heat the water again, and once
		// it's all gone the season starts over.
		l.Frost = max(l.Frost-airC, 0)
		l.Thaw = 0
	}
	if l.Thaw > 0 && !l.Frozen() {
		l.Frost, l.Thaw = 0, 0
	}
}
