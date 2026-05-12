package render

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// roadHoverOffset lifts the road quad strip above the terrain surface.
// Generous enough to mask two known mismatches between the road quad's
// Y sampling and the actual rendered mesh:
//   - terrain mesh vertices are jittered ±1 m in XZ by terrainJitterXYZ,
//     so bilinear interpolation across them at a road-edge vertex
//     differs slightly from `InterpolatedSurfaceElevationAt` (which
//     uses unjittered grid positions). The mismatch scales with the
//     local surface gradient — biggest where residual snow remains on
//     a sampled cell, which is exactly where roads would otherwise
//     read as "buried".
//   - the chain-effects closest-sample query overestimates true
//     curve-perpDist by ~1 sample spacing, leaving a few cm of
//     residual snow at the inner-clearance boundary.
// 20 cm is well above either error source and still reads as a flat
// surface at the gameplay camera distance.
const roadHoverOffset = float32(0.20)

// Lane-marking geometry. Pattern is dash → gap → dash → gap along the
// chain's centreline; widths are tuned to read at a normal gameplay
// camera distance without dominating the road surface.
const (
	roadLaneDashLength  = float32(2.5)  // dash length along the road (metres)
	roadLaneDashGap     = float32(3.5)  // gap between dashes
	roadLaneHalfWidth   = float32(0.15) // dashed line is 30 cm wide
	roadLaneHoverOffset = float32(0.05) // sits 5 cm above the asphalt — large enough that small bilinear differences between the dash quad (sampled at centreline) and the road quad (sampled at edges) can't flip the depth comparison and make dashes blink.
)

// generateRoadsMesh builds a single quad-strip mesh covering every road
// chain in the world. Each chain is sampled as a Catmull-Rom spline
// through its degree-2 interior nodes, so visually adjacent edges blend
// into a smooth curve instead of meeting at a hard corner.
//
// Returns nil if there are no chains.
func generateRoadsMesh(w *world.World, t *world.Terrain) *Mesh {
	chains := w.FindRoadChains()
	if len(chains) == 0 {
		return nil
	}
	verts := make([]float32, 0, 512)
	indices := make([]uint32, 0, 256)
	var baseIdx uint32
	for _, chain := range chains {
		v, idx := buildRoadChainStripVerts(chain, t, baseIdx)
		verts = append(verts, v...)
		indices = append(indices, idx...)
		baseIdx += uint32(len(v) / 8)
	}
	if len(indices) == 0 {
		return nil
	}
	return NewMesh(verts, indices, []int{3, 3, 2}, nil)
}

// generateRoadEdgeMesh builds a single straight-segment quad strip for
// the ghost preview path. Stays linear by design: the placement ghost
// is cheap-and-cheerful, and the final committed mesh re-curves through
// the chain on the next RebuildRoads.
func generateRoadEdgeMesh(a, b mgl32.Vec2, t *world.Terrain) *Mesh {
	samples := []mgl32.Vec2{a, b}
	v, idx := buildRoadStripFromSamples(samples, t, 0)
	if len(idx) == 0 {
		return nil
	}
	return NewMesh(v, idx, []int{3, 3, 2}, nil)
}

// generateRoadLanesMesh builds the dashed centreline mesh for every
// chain in the world. Drawn in a second pass with a near-white tint
// on top of the asphalt quad.
func generateRoadLanesMesh(w *world.World, t *world.Terrain) *Mesh {
	chains := w.FindRoadChains()
	if len(chains) == 0 {
		return nil
	}
	verts := make([]float32, 0, 512)
	indices := make([]uint32, 0, 256)
	var baseIdx uint32
	for _, chain := range chains {
		v, idx := buildRoadChainDashes(chain, t, baseIdx)
		verts = append(verts, v...)
		indices = append(indices, idx...)
		baseIdx += uint32(len(v) / 8)
	}
	if len(indices) == 0 {
		return nil
	}
	return NewMesh(verts, indices, []int{3, 3, 2}, nil)
}

// buildRoadChainStripVerts emits the asphalt quad strip for one chain.
// Catmull-Rom samples become the strip's centreline; perpendiculars are
// taken from the per-sample tangent so the strip width stays
// perpendicular to the curve even on tight bends.
func buildRoadChainStripVerts(chain world.RoadChain, t *world.Terrain, baseIdx uint32) ([]float32, []uint32) {
	samples := world.SampleRoadChain(chain, t, world.RoadChainSamplesPerSegment)
	return buildRoadStripFromSamples(samples, t, baseIdx)
}

// buildRoadStripFromSamples produces a quad strip walking through the
// supplied centreline samples. Shared between the chain path (which
// samples a curve) and the ghost preview path (which uses just two
// endpoint samples for a straight stub).
//
// Y is sampled at each edge vertex independently so the strip always
// sits roadHoverOffset above whatever the local terrain mesh is doing.
func buildRoadStripFromSamples(samples []mgl32.Vec2, t *world.Terrain, baseIdx uint32) ([]float32, []uint32) {
	if len(samples) < 2 {
		return nil, nil
	}
	halfWidth := world.RoadHalfWidth

	verts := make([]float32, 0, len(samples)*2*8)
	cumDist := world.CumulativeChainDist(samples)
	totalLen := cumDist[len(cumDist)-1]
	if totalLen < 0.5 {
		return nil, nil
	}

	// Emit two vertices per sample. Skip samples whose tangent is
	// degenerate (back-to-back coincident points) so we don't emit a
	// zero-length perpendicular.
	emittedPairs := 0
	for i := 0; i < len(samples); i++ {
		tx, tz := sampleTangent(samples, i)
		tlen := float32(math.Sqrt(float64(tx*tx + tz*tz)))
		if tlen < 1e-3 {
			continue
		}
		tx /= tlen
		tz /= tlen
		perpX := -tz * halfWidth
		perpZ := tx * halfWidth

		cx := samples[i][0]
		cz := samples[i][1]
		lx := cx - perpX
		lz := cz - perpZ
		ly := t.InterpolatedSurfaceElevationAt(lx, lz) + roadHoverOffset
		rx := cx + perpX
		rz := cz + perpZ
		ry := t.InterpolatedSurfaceElevationAt(rx, rz) + roadHoverOffset

		frac := cumDist[i] / totalLen
		verts = append(verts,
			lx, ly, lz, 0, 1, 0, frac, 0,
			rx, ry, rz, 0, 1, 0, frac, 1,
		)
		emittedPairs++
	}
	if emittedPairs < 2 {
		return nil, nil
	}

	indices := make([]uint32, 0, (emittedPairs-1)*6)
	for i := 0; i < emittedPairs-1; i++ {
		base := baseIdx + uint32(i*2)
		indices = append(indices,
			base, base+1, base+2,
			base+1, base+3, base+2,
		)
	}
	return verts, indices
}

// sampleTangent returns the (un-normalised) tangent direction at
// samples[i] using central differences in the interior and one-sided
// differences at the endpoints. Caller normalises.
func sampleTangent(samples []mgl32.Vec2, i int) (tx, tz float32) {
	switch {
	case i == 0:
		tx = samples[1][0] - samples[0][0]
		tz = samples[1][1] - samples[0][1]
	case i == len(samples)-1:
		tx = samples[i][0] - samples[i-1][0]
		tz = samples[i][1] - samples[i-1][1]
	default:
		tx = samples[i+1][0] - samples[i-1][0]
		tz = samples[i+1][1] - samples[i-1][1]
	}
	return
}

// buildRoadChainDashes emits the dashed centreline quads for one chain.
// Dashes are spaced uniformly along the chain's arc length (cumDist),
// so curve length, not Euclidean distance between nodes, drives the
// pattern. Each dash quad is small enough that its endpoints can share
// the closest sample's tangent without visible bending.
func buildRoadChainDashes(chain world.RoadChain, t *world.Terrain, baseIdx uint32) ([]float32, []uint32) {
	samples := world.SampleRoadChain(chain, t, world.RoadChainSamplesPerSegment)
	if len(samples) < 2 {
		return nil, nil
	}
	cumDist := world.CumulativeChainDist(samples)
	totalLen := cumDist[len(cumDist)-1]
	if totalLen < roadLaneDashLength {
		return nil, nil
	}

	period := roadLaneDashLength + roadLaneDashGap
	// Centre the pattern so a chain's first and last dash sit symmetrically.
	startOff := (totalLen - float32(int(totalLen/period))*period - roadLaneDashLength) / 2
	if startOff < 0 {
		startOff = 0
	}

	verts := make([]float32, 0, 256)
	indices := make([]uint32, 0, 64)
	var idx uint32
	dashOff := roadHoverOffset + roadLaneHoverOffset

	for s := startOff; s+roadLaneDashLength <= totalLen; s += period {
		p0 := pointAlongChain(samples, cumDist, s)
		p1 := pointAlongChain(samples, cumDist, s+roadLaneDashLength)
		tan0 := tangentAlongChain(samples, cumDist, s)
		tan1 := tangentAlongChain(samples, cumDist, s+roadLaneDashLength)

		perp0X := -tan0[1] * roadLaneHalfWidth
		perp0Z := tan0[0] * roadLaneHalfWidth
		perp1X := -tan1[1] * roadLaneHalfWidth
		perp1Z := tan1[0] * roadLaneHalfWidth

		x0L, z0L := p0[0]-perp0X, p0[1]-perp0Z
		x0R, z0R := p0[0]+perp0X, p0[1]+perp0Z
		x1L, z1L := p1[0]-perp1X, p1[1]-perp1Z
		x1R, z1R := p1[0]+perp1X, p1[1]+perp1Z

		quadBase := baseIdx + idx
		verts = append(verts,
			x0L, t.InterpolatedSurfaceElevationAt(x0L, z0L)+dashOff, z0L, 0, 1, 0, 0, 0,
			x0R, t.InterpolatedSurfaceElevationAt(x0R, z0R)+dashOff, z0R, 0, 1, 0, 0, 1,
			x1L, t.InterpolatedSurfaceElevationAt(x1L, z1L)+dashOff, z1L, 0, 1, 0, 1, 0,
			x1R, t.InterpolatedSurfaceElevationAt(x1R, z1R)+dashOff, z1R, 0, 1, 0, 1, 1,
		)
		indices = append(indices,
			quadBase, quadBase+1, quadBase+2,
			quadBase+1, quadBase+3, quadBase+2,
		)
		idx += 4
	}
	return verts, indices
}

// pointAlongChain returns the XZ position at arc length `dist` along
// the sample polyline. Clamps to endpoints outside [0, totalLen].
func pointAlongChain(samples []mgl32.Vec2, cumDist []float32, dist float32) mgl32.Vec2 {
	n := len(samples)
	if dist <= 0 {
		return samples[0]
	}
	if dist >= cumDist[n-1] {
		return samples[n-1]
	}
	for i := 1; i < n; i++ {
		if cumDist[i] >= dist {
			segLen := cumDist[i] - cumDist[i-1]
			if segLen < 1e-6 {
				return samples[i-1]
			}
			t := (dist - cumDist[i-1]) / segLen
			return mgl32.Vec2{
				samples[i-1][0] + (samples[i][0]-samples[i-1][0])*t,
				samples[i-1][1] + (samples[i][1]-samples[i-1][1])*t,
			}
		}
	}
	return samples[n-1]
}

// tangentAlongChain returns the unit tangent direction at arc length
// `dist`. Reads the local sub-segment direction rather than the
// underlying Catmull-Rom derivative — fine resolution makes the
// difference invisible at gameplay camera distance.
func tangentAlongChain(samples []mgl32.Vec2, cumDist []float32, dist float32) mgl32.Vec2 {
	n := len(samples)
	idx := 1
	if dist >= cumDist[n-1] {
		idx = n - 1
	} else if dist > 0 {
		for i := 1; i < n; i++ {
			if cumDist[i] >= dist {
				idx = i
				break
			}
		}
	}
	dx := samples[idx][0] - samples[idx-1][0]
	dz := samples[idx][1] - samples[idx-1][1]
	l := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if l < 1e-6 {
		return mgl32.Vec2{1, 0}
	}
	return mgl32.Vec2{dx / l, dz / l}
}
