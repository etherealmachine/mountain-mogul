package render

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Camera is a fixed-angle orthographic camera.
type Camera struct {
	Target     mgl32.Vec3
	Pitch      float32 // fixed: 45.0
	Yaw        float32 // fixed: 225.0
	OrthoScale float32 // world units per half-viewport-height; start at 150.0
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
	pitchRad := float64(c.Pitch) * math.Pi / 180.0
	yawRad := float64(c.Yaw) * math.Pi / 180.0

	distance := float64(c.OrthoScale)

	// Eye position relative to target
	eyeX := float32(distance * math.Cos(pitchRad) * math.Sin(yawRad))
	eyeY := float32(distance * math.Sin(pitchRad))
	eyeZ := float32(distance * math.Cos(pitchRad) * math.Cos(yawRad))

	eye := c.Target.Add(mgl32.Vec3{eyeX, eyeY, eyeZ})
	up := mgl32.Vec3{0, 1, 0}

	c.View = mgl32.LookAtV(eye, c.Target, up)

	aspect := c.viewport[0] / c.viewport[1]
	s := c.OrthoScale
	c.Proj = mgl32.Ortho(-aspect*s, aspect*s, -s, s, -10000, 10000)
}

// ViewProj returns the combined view-projection matrix.
func (c *Camera) ViewProj() mgl32.Mat4 {
	return c.Proj.Mul4(c.View)
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
