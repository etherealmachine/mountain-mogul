//go:build !demo

package scene

import (
	"math"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/world"
)

// SkiFree Mode (notes/next/SkiFree Mode.md): "Ski it" in a lift's popup
// drops the player's skier at its top (sim.StartPlayer) and follows them;
// the game runs at 1× around them. A/D or the arrows turn (Shift to skid
// round sharp), S or down brakes, space stops hard, W or up poles along;
// Esc hands control back, at the pace the game had.

// skiMode is the scenario's SkiFree state.
type skiMode struct {
	wasPaused bool // the game was paused when skiing began
	// The chase camera: the bearing it looks along (eased toward the
	// skier's heading), whether that's been set, and how far behind it
	// sits (scroll).
	chaseYaw  float32
	chaseSet  bool
	chaseDist float32
	scroll    float32 // this frame's scroll, for the chase camera
}

// The chase camera sits chaseBehind metres behind the skier (scroll
// moves it between chaseNear and chaseFar), chaseRise of that above
// them, looking at the snow chaseAhead metres down their line, so the
// slope ahead fills the view. It swings round behind them with a time
// constant of chaseEase seconds, and keeps chaseClear over the ground.
const (
	chaseBehind = float32(9)
	chaseNear   = float32(4)
	chaseFar    = float32(40)
	chaseRise   = float32(0.4)
	chaseAhead  = float32(14)
	chaseEase   = 0.45
	chaseClear  = float32(1.5)
	chaseFOV    = float32(65)
)

// startSkiing puts the player's skier at lift l's top and follows them.
func (s *Scenario) startSkiing(l *world.Lift) {
	if s.world.Player != nil {
		s.stopSkiing()
	}
	if s.popup != nil {
		s.popup.Visible = false
	}
	s.cancelTool()
	p := s.sim.StartPlayer(l)
	s.ski.wasPaused, s.paused = s.paused, false
	s.syncSpeedButtons()
	s.setFollowGuest(p.ID)
	s.ski.chaseSet = false
	if s.ski.chaseDist == 0 {
		s.ski.chaseDist = chaseBehind
	}
	s.setToast("Skiing: A/D turn (Shift: sharp), S brake, space stop, W pole along. Esc to stop.")
}

// stopSkiing takes the player's skier off and gives the game back.
func (s *Scenario) stopSkiing() {
	s.sim.StopPlayer()
	s.setFollowGuest(0)
	s.paused = s.ski.wasPaused
	s.syncSpeedButtons()
}

// skiInput reads the player's keys while they ski, and Esc to stop;
// reports whether they're skiing (so the camera keys stand down) and
// whether it used Esc.
func (s *Scenario) skiInput(inp *engine.Input, typing bool) (skiing, usedEsc bool) {
	if s.world.Player == nil {
		return false, false
	}
	if inp.Pressed[glfw.KeyEscape] {
		s.stopSkiing()
		return false, true
	}
	held := func(keys ...glfw.Key) bool {
		for _, k := range keys {
			if inp.Held[k] {
				return true
			}
		}
		return false
	}
	if typing {
		s.sim.PlayerInput = sim.PlayerInput{}
		return true, false
	}
	s.sim.PlayerInput = sim.PlayerInput{
		Left:  held(glfw.KeyA, glfw.KeyLeft),
		Right: held(glfw.KeyD, glfw.KeyRight),
		Sharp: held(glfw.KeyLeftShift, glfw.KeyRightShift),
		Brake: held(glfw.KeyS, glfw.KeyDown),
		Stop:  held(glfw.KeySpace),
		Push:  held(glfw.KeyW, glfw.KeyUp),
	}
	return true, false
}

// skiItButton adds "Ski it" to a lift's popup.
func (s *Scenario) skiItButton(w interface{ AddActionButton(string, func()) }, l *world.Lift) {
	w.AddActionButton("Ski it", func() { s.startSkiing(l) })
}

// chaseCamera puts the camera behind and above the player's skier,
// swinging round behind them as they turn (eased over chaseEase), looking
// down the slope ahead.
func (s *Scenario) chaseCamera(cam *render.Camera, p *world.Guest, dt float64, scroll float32) {
	st := &s.ski
	if scroll != 0 {
		st.chaseDist = min(max(st.chaseDist*float32(math.Pow(0.85, float64(scroll))), chaseNear), chaseFar)
	}
	if !st.chaseSet {
		st.chaseYaw, st.chaseSet = p.Heading, true
	}
	k := float32(1 - math.Exp(-dt/chaseEase))
	st.chaseYaw = wrapAngle(st.chaseYaw + wrapAngle(p.Heading-st.chaseYaw)*k)
	fwd := mgl32.Vec3{float32(math.Sin(float64(st.chaseYaw))), 0, float32(math.Cos(float64(st.chaseYaw)))}
	t := s.world.Terrain
	at := mgl32.Vec3{p.Pos[0], render.VisualElevationAt(t, p.Pos[0], p.Pos[2]), p.Pos[2]}
	eye := at.Sub(fwd.Mul(st.chaseDist))
	eye[1] = max(at[1]+st.chaseDist*chaseRise+1.5, render.VisualElevationAt(t, eye[0], eye[2])+chaseClear)
	look := at.Add(fwd.Mul(chaseAhead))
	look[1] = render.VisualElevationAt(t, look[0], look[2]) + 1
	cam.Perspective = true
	cam.FOVDeg = chaseFOV
	cam.EyePos = eye
	cam.LookAt = look
	cam.Recalculate()
}

// wrapAngle is a in (−π, π].
func wrapAngle(a float32) float32 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a <= -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

// SkiFrom starts skiing from the top of the i'th lift (for -screenshot).
func (s *Scenario) SkiFrom(i int) {
	if i >= 0 && i < len(s.world.Lifts) {
		s.startSkiing(s.world.Lifts[i])
	}
}
