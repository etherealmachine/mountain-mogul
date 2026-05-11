package render

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Camera is a fixed-angle orthographic camera with an opt-in
// first-person perspective mode for follow-skier views. When
// Perspective is true, EyePos / LookAt / FOVDeg drive the view and the
// existing ortho fields (Target, Pitch, Yaw, OrthoScale) are ignored.
type Camera struct {
	Target     mgl32.Vec3
	Pitch      float32 // fixed: 45.0
	Yaw        float32 // fixed: 225.0
	OrthoScale float32 // world units per half-viewport-height; start at 150.0

	// Perspective mode (first-person follow).
	Perspective bool
	EyePos      mgl32.Vec3
	LookAt      mgl32.Vec3
	FOVDeg      float32 // vertical FOV in degrees; defaults to 70 if 0

	viewport   mgl32.Vec2
	View, Proj mgl32.Mat4
}

// NewCamera creates a default camera.
func NewCamera(viewportW, viewportH int) *Camera {
	c := &Camera{
		Target:     mgl32.Vec3{0, 0, 0},
		Pitch:      45.0,
		Yaw:        225.0,
		OrthoScale: 150.0,
		viewport:   mgl32.Vec2{float32(viewportW), float32(viewportH)},
	}
	c.Recalculate()
	return c
}

// SetViewport updates the viewport size and recalculates matrices.
func (c *Camera) SetViewport(w, h int) {
	c.viewport = mgl32.Vec2{float32(w), float32(h)}
	c.Recalculate()
}

// Recalculate recomputes View and Proj matrices from current state.
func (c *Camera) Recalculate() {
	up := mgl32.Vec3{0, 1, 0}
	aspect := c.viewport[0] / c.viewport[1]

	if c.Perspective {
		fov := c.FOVDeg
		if fov <= 0 {
			fov = 70
		}
		c.View = mgl32.LookAtV(c.EyePos, c.LookAt, up)
		c.Proj = mgl32.Perspective(mgl32.DegToRad(fov), aspect, 0.1, 5000)
		return
	}

	pitchRad := float64(c.Pitch) * math.Pi / 180.0
	yawRad := float64(c.Yaw) * math.Pi / 180.0

	distance := float64(c.OrthoScale)

	// Eye position relative to target
	eyeX := float32(distance * math.Cos(pitchRad) * math.Sin(yawRad))
	eyeY := float32(distance * math.Sin(pitchRad))
	eyeZ := float32(distance * math.Cos(pitchRad) * math.Cos(yawRad))

	eye := c.Target.Add(mgl32.Vec3{eyeX, eyeY, eyeZ})

	c.View = mgl32.LookAtV(eye, c.Target, up)

	s := c.OrthoScale
	c.Proj = mgl32.Ortho(-aspect*s, aspect*s, -s, s, -10000, 10000)
}

// ViewProj returns the combined view-projection matrix.
func (c *Camera) ViewProj() mgl32.Mat4 {
	return c.Proj.Mul4(c.View)
}

// WorldPos returns the camera's world-space position. In perspective mode
// that's EyePos directly; in ortho mode it's reconstructed from yaw/pitch
// and OrthoScale (mirroring Recalculate's eye math). Used by shaders that
// need a true view direction per fragment, e.g. snow sparkle.
func (c *Camera) WorldPos() mgl32.Vec3 {
	if c.Perspective {
		return c.EyePos
	}
	pitchRad := float64(c.Pitch) * math.Pi / 180.0
	yawRad := float64(c.Yaw) * math.Pi / 180.0
	d := float64(c.OrthoScale)
	return c.Target.Add(mgl32.Vec3{
		float32(d * math.Cos(pitchRad) * math.Sin(yawRad)),
		float32(d * math.Sin(pitchRad)),
		float32(d * math.Cos(pitchRad) * math.Cos(yawRad)),
	})
}

// PanDelta converts a 2D screen-space drag (pixels) to a world-space XZ
// translation for the camera target. Accounts for camera yaw so screen-right
// always corresponds to the camera's right direction regardless of orientation.
func (c *Camera) PanDelta(screenDelta mgl32.Vec2) (worldDX, worldDZ float32) {
	yawRad := float64(c.Yaw) * math.Pi / 180.0
	// Camera right is always horizontal (perpendicular to Y-axis)
	rightX := float32(math.Cos(yawRad))
	rightZ := float32(-math.Sin(yawRad))
	// Camera forward projected onto the horizontal plane
	fwdX := float32(-math.Sin(yawRad))
	fwdZ := float32(-math.Cos(yawRad))
	// World units per pixel
	scale := c.OrthoScale / c.viewport[1] * 2
	worldDX = (screenDelta[0]*rightX + screenDelta[1]*fwdX) * scale
	worldDZ = (screenDelta[0]*rightZ + screenDelta[1]*fwdZ) * scale
	return
}

// ScreenToWorldRay returns the ray origin and direction for a screen position.
// In orthographic projection, all rays are parallel (same direction).
func (c *Camera) ScreenToWorldRay(screenPos mgl32.Vec2) (origin, dir mgl32.Vec3) {
	// Convert screen coordinates to NDC
	ndcX := (2.0*screenPos[0])/c.viewport[0] - 1.0
	ndcY := 1.0 - (2.0*screenPos[1])/c.viewport[1]

	// Inverse of view-projection
	vp := c.ViewProj()
	vpInv := vp.Inv()

	// Unproject near and far points
	near := mgl32.TransformCoordinate(mgl32.Vec3{ndcX, ndcY, -1.0}, vpInv)
	far := mgl32.TransformCoordinate(mgl32.Vec3{ndcX, ndcY, 1.0}, vpInv)

	origin = near
	dir = far.Sub(near).Normalize()
	return
}

// ScreenToTerrain projects a screen position onto the horizontal plane y=terrainY.
func (c *Camera) ScreenToTerrain(screenPos mgl32.Vec2, terrainY float32) mgl32.Vec3 {
	origin, dir := c.ScreenToWorldRay(screenPos)
	if math.Abs(float64(dir[1])) < 1e-6 {
		return origin
	}
	t := (terrainY - origin[1]) / dir[1]
	return origin.Add(dir.Mul(t))
}

// ScreenCenterOnHeightmap returns the world-space point at the centre of
// the screen that lies on the heightmap given by elevationAt(x, z). Used
// to pivot camera rotation around what the player actually sees in the
// middle of the view, not the abstract Target anchor — which sits at
// whatever Y the scene last placed it (often 0, far below the visible
// terrain on a mountain map).
//
// Iterates the screen-centre ray against a plane at the previous Y
// estimate, sampling the heightmap at the new XZ each round, until the
// plane height matches the heightmap. At ortho pitch 45° each iteration
// shrinks the residual by (1 - terrain_slope) so on resort terrain
// (slopes well under 45°) we converge geometrically in a handful of
// rounds — but the *seed* matters a lot at high zoom, because a cold
// Target.Y of 0 on a 200 m mountain takes several iterations to walk in
// from. We seed instead from the heightmap at Target.XZ, which is on
// the screen-centre ray from the previous frame (since we last snapped
// Target there), so cold-start error is bounded by terrain undulation
// over one rotation step. A 1 mm threshold and 16-iteration cap absorb
// the rare divergent geometry without visible jitter even when one
// world-metre fills tens of screen pixels.
func (c *Camera) ScreenCenterOnHeightmap(elevationAt func(x, z float32) float32) mgl32.Vec3 {
	centre := mgl32.Vec2{c.viewport[0] * 0.5, c.viewport[1] * 0.5}
	y := elevationAt(c.Target[0], c.Target[2])
	for i := 0; i < 16; i++ {
		p := c.ScreenToTerrain(centre, y)
		newY := elevationAt(p[0], p[2])
		if math.Abs(float64(newY-y)) < 0.001 {
			y = newY
			break
		}
		y = newY
	}
	// One final raycast with the converged Y so the returned XZ is
	// exactly on the screen-centre ray at that elevation (the loop's
	// last `p` was raycast against the *previous* Y).
	p := c.ScreenToTerrain(centre, y)
	p[1] = y
	return p
}
