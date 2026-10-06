package world

// Material is what the ground is made of under the snow. It sets the
// bare ground's look, and, as the snow sim learns to read it, how much
// snow the ground holds and whether trees grow there.
type Material uint8

// Auto material only sets rock and meadow today. Dirt and scree are
// for a material brush and rock palettes later.
const (
	MatMeadow Material = iota // grass and low scrub; the height-driven ground colour
	MatDirt                   // steep soil and bare earth
	MatScree                  // loose rock below cliffs
	MatRock                   // solid rock
	NumMaterials
)

var materialNames = [NumMaterials]string{"meadow", "dirt", "scree", "rock"}

func (m Material) String() string {
	if m < NumMaterials {
		return materialNames[m]
	}
	return "unknown"
}

// IsRock reports whether m is solid rock.
func (m Material) IsRock() bool { return m == MatRock }

// Bare reports whether nothing grows on m: rock and scree.
func (m Material) Bare() bool { return m.IsRock() || m == MatScree }

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
