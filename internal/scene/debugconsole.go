package scene

import (
	"strings"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// DebugConsole is a tilde-toggled cheat-code console.
type DebugConsole struct {
	visible      bool
	input        *ui.TextInput
	world        *world.World
	toast        func(string)
	sim          *sim.Simulation
	flushTerrain func()
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

// SetSim gives the console access to the running simulation and a callback to
// flush terrain vertices after weather cheats modify the snow layer stack.
// Called each time a new simulation is started (new game, load, testbed).
func (c *DebugConsole) SetSim(s *sim.Simulation, flushTerrain func()) {
	c.sim = s
	c.flushTerrain = flushTerrain
}

func (c *DebugConsole) Visible() bool  { return c.visible }
func (c *DebugConsole) Toggle()        { c.visible = !c.visible }
func (c *DebugConsole) Show()          { c.visible = true }

func (c *DebugConsole) HandleInput(inp *engine.Input) {
	c.input.HandleInput(inp)
}

func (c *DebugConsole) exec(cmd string) {
	switch strings.ToLower(cmd) {
	case "moremoney":
		c.world.Cash += 100_000
		c.toast("+$100,000")
	case "moresnow":
		if c.sim == nil {
			return
		}
		c.sim.TriggerStorm()
		if c.flushTerrain != nil {
			c.flushTerrain()
		}
		c.toast("Storm incoming!")
	case "heatwave":
		if c.sim == nil {
			return
		}
		c.sim.TriggerHeatwave()
		if c.flushTerrain != nil {
			c.flushTerrain()
		}
		c.toast("Heat wave!")
	}
}

func (c *DebugConsole) Draw(r *render.Renderer) {
	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	defer gl.Disable(gl.BLEND)

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
