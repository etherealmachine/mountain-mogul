package world

// Material is what the ground is made of under the snow. It sets the
// bare ground's look, and, as the snow sim learns to read it, how much
// snow the ground holds and whether trees grow there.
type Material uint8

// Auto material sets rock, lake surfaces, and meadow today. Dirt and
// scree are for a material brush and rock palettes later. A lake's
// surface (ice, thin ice, open water) follows its ice through the season.
const (
	MatMeadow    Material = iota // grass and low scrub; the height-driven ground colour
	MatDirt                      // steep soil and bare earth
	MatScree                     // loose rock below cliffs
	MatRock                      // solid rock
	MatIce                       // a frozen lake or pond, under the snow
	MatThinIce                   // a lake freezing or thawing: dark ice, little snow
	MatOpenWater                 // an open lake: snow falling on it melts in
	NumMaterials
)

var materialNames = [NumMaterials]string{"meadow", "dirt", "scree", "rock", "ice", "thin ice", "open water"}

func (m Material) String() string {
	if m < NumMaterials {
		return materialNames[m]
	}
	return "unknown"
}

// IsRock reports whether m is solid rock.
func (m Material) IsRock() bool { return m == MatRock }

// IsWater reports whether m is a lake's surface, frozen or not.
func (m Material) IsWater() bool { return m == MatIce || m == MatThinIce || m == MatOpenWater }

// Bare reports whether nothing grows on m: rock, scree, and water.
func (m Material) Bare() bool { return m.IsRock() || m == MatScree || m.IsWater() }

// TerrainMaterial is the ground's material on the detail lattice: sample
// (i, j) sits at corner-grid coordinate (i/DetailPerCell,
// j/DetailPerCell), world (i, j) × CellSize/DetailPerCell, the same
// points as TerrainDetail.
type TerrainMaterial struct {
	W, H int        // samples: (Width-1)*DetailPerCell+1 × (Height-1)*DetailPerCell+1
	M    []Material // row-major: M[j*W+i]
	// Version counts edits made in place, so the renderer knows to
	// upload the map again.
	Version int
}

// NewTerrainMaterial makes an all-meadow map for a terrain of
// wCells × hCells.
func NewTerrainMaterial(wCells, hCells int) *TerrainMaterial {
	w := (wCells-1)*DetailPerCell + 1
	h := (hCells-1)*DetailPerCell + 1
	return &TerrainMaterial{W: w, H: h, M: make([]Material, w*h)}
}

// At is the material of the sample nearest world position (wx, wz).
func (m *TerrainMaterial) At(wx, wz float32) Material {
	const per = CellSize / DetailPerCell
	i := min(max(int(wx/per+0.5), 0), m.W-1)
	j := min(max(int(wz/per+0.5), 0), m.H-1)
	return m.M[j*m.W+i]
}

// Bytes is the map for saving, one byte per sample.
func (m *TerrainMaterial) Bytes() []byte {
	b := make([]byte, len(m.M))
	for k, v := range m.M {
		b[k] = byte(v)
	}
	return b
}

// LoadTerrainMaterial reads a map written by Bytes for a terrain of
// wCells × hCells, or nil when it doesn't fit.
func LoadTerrainMaterial(wCells, hCells int, b []byte) *TerrainMaterial {
	m := NewTerrainMaterial(wCells, hCells)
	if len(b) != len(m.M) {
		return nil
	}
	for k, v := range b {
		m.M[k] = Material(min(v, byte(NumMaterials-1)))
	}
	return m
}
