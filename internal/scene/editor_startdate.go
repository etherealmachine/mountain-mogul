//go:build !demo

package scene

import (
	"fmt"
	"math"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/settings"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// startDatePanel is the editor's Start control: month, day and year
// steppers for World.StartDate and an hour stepper for World.StartHour,
// drawn as a strip centred at the top of the screen. get/set read and write the
// date, getHour/setHour the hour; the setters also mark the scenario
// dirty.
type startDatePanel struct {
	get     func() time.Time
	set     func(time.Time)
	getHour func() float32
	setHour func(float32)
	btns    []*ui.Button // month −/+, day −/+, year −/+, hour −/+
	top     float32      // y of the strip's anchor

	x, y, w, h float32 // background rect, from the last layout
	labelX     [4]float32
}

const (
	startDateBtnW = float32(26)
	startDatePad  = float32(8)
)

// startDateFieldW is the width of value slot i between its steppers:
// room for "2026" or "Sep", or "10:00 AM" for the hour, in the loaded
// font.
func startDateFieldW(i int) float32 {
	if i == 3 {
		return 7*float32(render.GlyphAdvance) + startDatePad
	}
	return 4*float32(render.GlyphAdvance) + startDatePad
}

func newStartDatePanel(top float32, get func() time.Time, set func(time.Time),
	getHour func() float32, setHour func(float32)) *startDatePanel {
	p := &startDatePanel{get: get, set: set, getHour: getHour, setHour: setHour, top: top}
	step := func(years, months, days int) func() {
		return func() { p.set(stepStartDate(p.get(), years, months, days)) }
	}
	hour := func(d float32) func() {
		return func() { p.setHour(float32(math.Mod(float64(p.getHour()+d)+24, 24))) }
	}
	for _, f := range []func(){
		step(0, -1, 0), step(0, 1, 0),
		step(0, 0, -1), step(0, 0, 1),
		step(-1, 0, 0), step(1, 0, 0),
		hour(-1), hour(1),
	} {
		label := "<"
		if len(p.btns)%2 == 1 {
			label = ">"
		}
		p.btns = append(p.btns, ui.NewButton(0, 0, startDateBtnW, 0, label, f))
	}
	return p
}

// stepStartDate moves d by the given years, months or game days. Month and
// year steps keep the game day of the month.
func stepStartDate(d time.Time, years, months, days int) time.Time {
	if days != 0 {
		return world.GameDayAt(world.GameDayIndex(d) + days)
	}
	first := time.Date(d.Year()+years, d.Month()+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	return world.GameDayStart(first.Year(), first.Month(), world.GameDay(d))
}

// layout centres the strip horizontally just below top.
func (p *startDatePanel) layout(screenW float32) {
	caption := 6 * float32(render.GlyphAdvance) // "Start" plus a gap
	btnH := float32(render.GlyphH) + 6
	p.w = startDatePad + caption + startDatePad
	for i := range p.labelX {
		p.w += 2*startDateBtnW + startDateFieldW(i) + startDatePad
	}
	p.h = btnH + 2*6
	p.x = (screenW - p.w) / 2
	p.y = p.top + 6
	x := p.x + startDatePad + caption
	by := p.y + 6
	for _, b := range p.btns {
		b.Y, b.H = by, btnH
	}
	for i := range p.labelX {
		p.btns[2*i].X = x
		p.labelX[i] = x + startDateBtnW
		p.btns[2*i+1].X = x + startDateBtnW + startDateFieldW(i)
		x += 2*startDateBtnW + startDateFieldW(i) + startDatePad
	}
}

// Contains reports whether the point is over the strip.
func (p *startDatePanel) Contains(mx, my float32) bool {
	return mx >= p.x && mx <= p.x+p.w && my >= p.y && my <= p.y+p.h
}

// HandleInput updates hover state and fires a stepper on click, consuming
// the click so it doesn't reach the world tools.
func (p *startDatePanel) HandleInput(inp *engine.Input, screenW float32) {
	p.layout(screenW)
	mx, my := inp.MousePos[0], inp.MousePos[1]
	for _, b := range p.btns {
		b.SetHovered(b.Contains(mx, my))
	}
	if !inp.LeftClick || inp.LeftClickConsumed || !p.Contains(mx, my) {
		return
	}
	inp.LeftClickConsumed = true
	for _, b := range p.btns {
		if b.Contains(mx, my) {
			b.Click()
			return
		}
	}
}

func (p *startDatePanel) Draw(r *render.Renderer) {
	p.layout(float32(r.ScreenWidth()))
	ui.DrawPanel(r, p.x, p.y, p.w, p.h)
	for _, b := range p.btns {
		b.Draw(r)
	}
	if r.Font == nil {
		return
	}
	col := mgl32.Vec4{0.9, 0.95, 1, 1}
	textY := p.y + (p.h-float32(render.GlyphH))/2
	r.Font.DrawText(r, "Start", p.x+startDatePad, textY, col)
	d := p.get()
	for i, s := range []string{d.Format("Jan"), fmt.Sprint(world.GameDay(d)), d.Format("2006"),
		settings.FormatClock(float64(p.getHour()))} {
		tw := r.Font.TextWidth(s)
		r.Font.DrawText(r, s, p.labelX[i]+(startDateFieldW(i)-tw)/2, textY, col)
	}
}
