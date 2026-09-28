package scene

import (
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
)

// startDatePanel is the editor's Start date control: month, day and year
// steppers for World.StartDate, drawn as a strip centred under the top bar.
// get/set read and write the date; set also marks the scenario dirty.
type startDatePanel struct {
	get  func() time.Time
	set  func(time.Time)
	btns []*ui.Button // month −/+, day −/+, year −/+
	top  float32      // y of the strip's anchor (the top bar's bottom edge)

	x, y, w, h float32 // background rect, from the last layout
	labelX     [3]float32
}

const (
	startDateBtnW = float32(26)
	startDatePad  = float32(8)
)

// startDateFieldW is the width of each value slot between its steppers:
// room for "2026" or "Sep" in the loaded font.
func startDateFieldW() float32 { return 4*float32(render.GlyphAdvance) + startDatePad }

func newStartDatePanel(top float32, get func() time.Time, set func(time.Time)) *startDatePanel {
	p := &startDatePanel{get: get, set: set, top: top}
	step := func(years, months, days int) func() {
		return func() { p.set(stepStartDate(p.get(), years, months, days)) }
	}
	for _, f := range []func(){
		step(0, -1, 0), step(0, 1, 0),
		step(0, 0, -1), step(0, 0, 1),
		step(-1, 0, 0), step(1, 0, 0),
	} {
		label := "<"
		if len(p.btns)%2 == 1 {
			label = ">"
		}
		p.btns = append(p.btns, ui.NewButton(0, 0, startDateBtnW, 0, label, f))
	}
	return p
}

// stepStartDate moves d by the given years, months or days. Month and year
// steps keep the day of the month, clamped to the target month's length
// (Jan 31 + 1 month = Feb 28), rather than time.AddDate's overflow.
func stepStartDate(d time.Time, years, months, days int) time.Time {
	if days != 0 {
		return d.AddDate(0, 0, days)
	}
	first := time.Date(d.Year()+years, d.Month()+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	lastDay := first.AddDate(0, 1, -1).Day()
	day := d.Day()
	if day > lastDay {
		day = lastDay
	}
	return first.AddDate(0, 0, day-1)
}

// layout centres the strip horizontally just below the top bar.
func (p *startDatePanel) layout(screenW float32) {
	caption := 11 * float32(render.GlyphAdvance) // "Start date" plus a gap
	fieldW := startDateFieldW()
	btnH := float32(render.GlyphH) + 6
	slotW := 2*startDateBtnW + fieldW
	p.w = startDatePad + caption + 3*slotW + 2*startDatePad + startDatePad
	p.h = btnH + 2*6
	p.x = (screenW - p.w) / 2
	p.y = p.top + 6
	x := p.x + startDatePad + caption
	by := p.y + 6
	for _, b := range p.btns {
		b.Y, b.H = by, btnH
	}
	for i := 0; i < 3; i++ {
		p.btns[2*i].X = x
		p.labelX[i] = x + startDateBtnW
		p.btns[2*i+1].X = x + startDateBtnW + fieldW
		x += slotW + startDatePad
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
	r.DrawColorRect(p.x, p.y, p.w, p.h, mgl32.Vec4{0.08, 0.10, 0.14, 0.92})
	for _, b := range p.btns {
		b.Draw(r)
	}
	if r.Font == nil {
		return
	}
	col := mgl32.Vec4{0.9, 0.95, 1, 1}
	textY := p.y + (p.h-float32(render.GlyphH))/2
	r.Font.DrawText(r, "Start date", p.x+startDatePad, textY, col)
	d := p.get()
	for i, s := range []string{d.Format("Jan"), d.Format("2"), d.Format("2006")} {
		tw := r.Font.TextWidth(s)
		r.Font.DrawText(r, s, p.labelX[i]+(startDateFieldW()-tw)/2, textY, col)
	}
}
