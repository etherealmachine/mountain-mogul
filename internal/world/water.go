package world

import "math"

// Water is a map's creeks and lakes as the ground is built from them: the
// Water terrain layer fits the import's OpenStreetMap water to the ground
// once (geo.FitWater) and keeps it here, where the scenario editor's
// Water tool changes it; the Creeks and Lakes layers carve and level
// what's here, never OpenStreetMap's. World metres (x, z), like the
// cells' corners.
type Water struct {
	Creeks []WaterCreek
	Lakes  []WaterLake
	// Fitted is set once the Water layer has fitted the water; until then
	// it fits it on its next run. Clearing it (and the water) refits from
	// OpenStreetMap, losing any changes.
	Fitted bool
}

// WaterType is what kind of water a creek or lake is, which sets what it
// does in winter. A creek's line is a Creek, snow-bridged all winter, or
// a River, open all winter. A lake's outline is a Pond, shallow, freezing
// all at once when the cold comes; a Lake, freezing from its shallows
// out over weeks and thawing in spring; or a River, Sea or Ocean, open
// all winter. The Water layer picks one from OpenStreetMap
// (CreekTypeOf, LakeTypeOf); the scenario editor's Water tool changes it.
type WaterType uint8

const (
	WaterTypeCreek WaterType = iota
	WaterTypeRiver
	WaterTypePond
	WaterTypeLake
	WaterTypeSea
	WaterTypeOcean
	WaterTypeCount
)

// CreekTypes and LakeTypes are the types a creek's line and a lake's
// outline can be, in the editor's order.
var (
	CreekTypes = []WaterType{WaterTypeCreek, WaterTypeRiver}
	LakeTypes  = []WaterType{WaterTypePond, WaterTypeLake, WaterTypeRiver, WaterTypeSea, WaterTypeOcean}
)

// Label is the type's name in the editor.
func (t WaterType) Label() string {
	switch t {
	case WaterTypeRiver:
		return "River"
	case WaterTypePond:
		return "Pond"
	case WaterTypeLake:
		return "Lake"
	case WaterTypeSea:
		return "Sea"
	case WaterTypeOcean:
		return "Ocean"
	}
	return "Creek"
}

// Open reports whether water of this type stays open all winter.
func (t WaterType) Open() bool {
	return t == WaterTypeRiver || t == WaterTypeSea || t == WaterTypeOcean
}

// pondMaxHa is the largest untagged lake taken for a pond, hectares.
const pondMaxHa = 2

// CreekTypeOf is the type of a creek with OpenStreetMap waterway value
// kind: a river or canal is a River, anything smaller a Creek.
func CreekTypeOf(kind string) WaterType {
	if kind == "river" || kind == "canal" {
		return WaterTypeRiver
	}
	return WaterTypeCreek
}

// LakeTypeOf is the type of a lake with OpenStreetMap water value kind
// and areaHa hectares: by its tag, else a Pond up to pondMaxHa and a Lake
// above.
func LakeTypeOf(kind string, areaHa float32) WaterType {
	switch kind {
	case "pond", "basin", "fishpond", "reflecting_pool", "wastewater":
		return WaterTypePond
	case "river", "canal", "stream":
		return WaterTypeRiver
	case "sea":
		return WaterTypeSea
	case "ocean":
		return WaterTypeOcean
	case "":
		if areaHa <= pondMaxHa {
			return WaterTypePond
		}
	}
	return WaterTypeLake
}

// WaterCreek is a creek's line from its source down.
type WaterCreek struct {
	Name, Kind string
	Points     [][2]float32
	Type       WaterType
}

// WaterLake is a lake's outline: closed rings, each ring's last point
// joining its first, a point inside an odd number of rings being water.
// AreaHa is the whole lake's area, off the map too, from OpenStreetMap.
type WaterLake struct {
	Name, Kind string
	AreaHa     float32
	Rings      [][][2]float32
	Type       WaterType
}

// Clone is a deep copy of w.
func (w *Water) Clone() *Water {
	out := &Water{Fitted: w.Fitted, Creeks: make([]WaterCreek, len(w.Creeks)), Lakes: make([]WaterLake, len(w.Lakes))}
	for i, c := range w.Creeks {
		c.Points = append([][2]float32(nil), c.Points...)
		out.Creeks[i] = c
	}
	for i, l := range w.Lakes {
		rings := make([][][2]float32, len(l.Rings))
		for k, r := range l.Rings {
			rings[k] = append([][2]float32(nil), r...)
		}
		l.Rings = rings
		out.Lakes[i] = l
	}
	return out
}

// creekCurveStep is the most metres between the points Curve returns.
const creekCurveStep = 1.0

// Curve is the creek's course: a smooth curve through its points (a
// centripetal Catmull–Rom spline, which passes through each point
// without overshooting or looping between unevenly spaced ones),
// returned as points at most creekCurveStep apart. The creek's points
// are where it's edited; everything that follows the creek follows this.
func (c WaterCreek) Curve() [][2]float32 {
	ps := c.Points
	if len(ps) < 3 {
		return append([][2]float32(nil), ps...)
	}
	out := make([][2]float32, 0, len(ps)*4)
	for i := 0; i+1 < len(ps); i++ {
		a, b := ps[i], ps[i+1]
		l := float32(math.Hypot(float64(b[0]-a[0]), float64(b[1]-a[1])))
		n := max(int(math.Ceil(float64(l/creekCurveStep))), 1)
		for k := 0; k < n; k++ {
			out = append(out, CurveAt(ps, i, float32(k)/float32(n)))
		}
	}
	return append(out, ps[len(ps)-1])
}

// CurveAt is the point a share t (0–1) of the way along the curve
// through ps from point i to point i+1 (WaterCreek.Curve).
func CurveAt(ps [][2]float32, i int, t float32) [2]float32 {
	p1, p2 := ps[i], ps[i+1]
	p0, p3 := p1, p2
	if i > 0 {
		p0 = ps[i-1]
	} else {
		p0 = [2]float32{2*p1[0] - p2[0], 2*p1[1] - p2[1]}
	}
	if i+2 < len(ps) {
		p3 = ps[i+2]
	} else {
		p3 = [2]float32{2*p2[0] - p1[0], 2*p2[1] - p1[1]}
	}
	knot := func(a, b [2]float32) float32 {
		d := math.Hypot(float64(b[0]-a[0]), float64(b[1]-a[1]))
		return float32(math.Max(math.Sqrt(d), 1e-3))
	}
	t0 := float32(0)
	t1 := t0 + knot(p0, p1)
	t2 := t1 + knot(p1, p2)
	t3 := t2 + knot(p2, p3)
	u := t1 + (t2-t1)*t
	lerp := func(a, b [2]float32, ta, tb float32) [2]float32 {
		w := (u - ta) / (tb - ta)
		return [2]float32{a[0] + (b[0]-a[0])*w, a[1] + (b[1]-a[1])*w}
	}
	a1, a2, a3 := lerp(p0, p1, t0, t1), lerp(p1, p2, t1, t2), lerp(p2, p3, t2, t3)
	b1, b2 := lerp(a1, a2, t0, t2), lerp(a2, a3, t1, t3)
	return lerp(b1, b2, t1, t2)
}
