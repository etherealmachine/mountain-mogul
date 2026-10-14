//go:build !demo

package scene

import (
	"strings"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// DebugConsole is the cheat-code console, toggled on ~ or opened by
// typing /. Commands take an optional leading slash: /sandbox, moremoney.
type DebugConsole struct {
	visible      bool
	input        *ui.TextInput
	world        *world.World
	toast        func(string)
	sim          *sim.Simulation
	flushTerrain func()
	flushStatic  func()
}

func newDebugConsole(w *world.World, toast func(string)) *DebugConsole {
	c := &DebugConsole{world: w, toast: toast}
	c.input = ui.NewTextInput(0, 0, 400, 36, "")
	c.input.MaxLen = 64
	c.input.OnSubmit = func(text string) {
		c.exec(strings.TrimSpace(text))
		c.input.Text = ""
		c.visible = false
	}
	c.input.OnCancel = func() {
		c.input.Text = ""
		c.visible = false
	}
	return c
}

// SetSim gives the console access to the running simulation, a callback to
// flush terrain vertices after weather cheats modify the snow layer stack,
// and one to rebuild static instances (trees) after ground edits.
// Called each time a new simulation is started (new game, load, testbed).
func (c *DebugConsole) SetSim(s *sim.Simulation, flushTerrain, flushStatic func()) {
	c.sim = s
	if s != nil {
		c.world = s.World
	}
	c.flushTerrain = flushTerrain
	c.flushStatic = flushStatic
}

func (c *DebugConsole) Visible() bool { return c.visible }
func (c *DebugConsole) Toggle()       { c.visible = !c.visible }
func (c *DebugConsole) Show()         { c.visible = true }

func (c *DebugConsole) HandleInput(inp *engine.Input) {
	c.input.HandleInput(inp)
}

func (c *DebugConsole) exec(cmd string) {
	switch strings.ToLower(strings.TrimPrefix(cmd, "/")) {
	case "":
	case "sandbox":
		c.world.Sandbox = !c.world.Sandbox
		if c.world.Sandbox {
			c.toast("Sandbox on: building is free")
		} else {
			c.toast("Sandbox off")
		}
	case "moremoney":
		c.world.Cash += 100_000
		c.toast("+$100,000")
	case "lessmoney":
		c.world.Cash -= 500_000
		c.toast("-$500,000")
	case "moresnow":
		if c.sim == nil {
			return
		}
		c.sim.TriggerStorm()
		if c.flushTerrain != nil {
			c.flushTerrain()
		}
		c.toast("Storm incoming!")
	case "groom":
		if c.sim == nil {
			return
		}
		c.sim.GroomAllNow()
		if c.flushTerrain != nil {
			c.flushTerrain()
		}
		c.toast("Groomed every section")
	case "heatwave":
		if c.sim == nil {
			return
		}
		c.sim.TriggerHeatwave()
		if c.flushTerrain != nil {
			c.flushTerrain()
		}
		c.toast("Heat wave!")
	case "avalanche":
		if c.sim == nil {
			return
		}
		c.sim.TriggerAvalanche()
		if c.flushTerrain != nil {
			c.flushTerrain()
		}
		c.toast("Avalanche!")
	case "regrade":
		RegradeEmbankments(c.world)
		if c.flushTerrain != nil {
			c.flushTerrain()
		}
		if c.flushStatic != nil {
			c.flushStatic()
		}
		c.toast("Embankments regraded")
	default:
		c.toast("Unknown command: " + cmd)
	}
}

func (c *DebugConsole) Draw(r *render.Renderer) {
	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())

	const w, h float32 = 400, 36
	x := (sw - w) / 2
	y := sh*0.65 - h/2

	c.input.X, c.input.Y, c.input.W, c.input.H = x, y, w, h
	c.input.Draw(r)

	if r.Font != nil {
		label := "> "
		lw := r.Font.TextWidth(label)
		r.Font.DrawText(r, label, x-lw-4, y+(h-float32(render.GlyphH))/2, mgl32.Vec4{0.6, 1, 0.6, 1})
	}
}
