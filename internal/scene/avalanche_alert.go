package scene

import (
	"strings"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/settings"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// checkAvalancheAlerts looks at what the sim logged this frame: a slide
// across the runs, or someone caught or killed, pauses the game (unless
// the player turned that off) and opens a window to go and look; control
// work bringing a slope down says so in a toast.
func (s *Scenario) checkAvalancheAlerts() {
	ev := &s.world.Events
	n := int(min(ev.Seq-s.avySeq, uint64(ev.Len())))
	s.avySeq = ev.Seq
	for i := n - 1; i >= 0; i-- { // oldest first
		e := ev.At(ev.Len() - 1 - i)
		switch e.Kind {
		case world.EventAvalanche:
			if e.HasPos {
				s.avalancheAlert(e)
			}
		case world.EventAvalancheControl:
			s.setToast(e.Message)
		}
	}
}

// avalancheAlert pauses for event e and opens its window, replacing one
// still on screen.
func (s *Scenario) avalancheAlert(e world.Event) {
	if !settings.Get().NoAvalanchePause && !s.noAlertPause && !s.paused {
		s.paused, s.avyPaused = true, true
		s.syncSpeedButtons()
	}
	win := ui.NewWindow("Avalanche!", 0, 0)
	msg := strings.TrimPrefix(e.Message, "Avalanche: ")
	win.AddSection(strings.ToUpper(msg[:1]) + msg[1:])
	resume := func() {
		win.Visible = false
		if s.avyPaused {
			s.paused, s.avyPaused = false, false
			s.syncSpeedButtons()
		}
	}
	at := e.Pos
	win.AddActionButton("Go to it", func() {
		s.jumpTo(at[0], at[1])
		resume()
	})
	win.AddActionButton("Carry on", resume)
	win.AddBoolToggle("Pause for avalanches",
		func() bool { return !settings.Get().NoAvalanchePause },
		func(on bool) {
			settings.Get().NoAvalanchePause = !on
			_ = settings.Save()
		})
	win.Visible = true
	r := s.app.Renderer
	s.placeWindow(win, r.ScreenWidth(), r.ScreenHeight(), &s.reportAt)
	s.avyAlert = win
}

// jumpTo drops any follow and centres the camera on world (x, z).
func (s *Scenario) jumpTo(x, z float32) {
	s.setFollowGuest(0)
	c := s.app.Renderer.Camera
	c.Target = mgl32.Vec3{x, s.world.Terrain.InterpolatedSurfaceElevationAt(x, z), z}
	c.Recalculate()
}
