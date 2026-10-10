package scene

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// scenarioMoney is the money a scenario starts with, as the details
// dialog's Money tab edits it: World's cash, credit line and prices.
type scenarioMoney struct {
	Cash, CreditLimit           int
	CreditRate                  float32
	DayTicket, SeasonPass, Park int
}

func moneyOf(w *world.World) scenarioMoney {
	return scenarioMoney{
		Cash: w.Cash, CreditLimit: w.CreditLimit, CreditRate: w.CreditRate,
		DayTicket: w.DayTicketPrice, SeasonPass: w.SeasonPassPrice, Park: w.ParkingPrice,
	}
}

func (m scenarioMoney) apply(w *world.World) {
	w.Cash, w.CreditLimit, w.CreditRate = m.Cash, m.CreditLimit, m.CreditRate
	w.DayTicketPrice, w.SeasonPassPrice, w.ParkingPrice = m.DayTicket, m.SeasonPass, m.Park
}

// moneyTab is the Money tab: a stepper per field, and the money the
// player can spend at the start.
type moneyTab struct {
	m    scenarioMoney
	rows []moneyRow
}

// moneyRow is one stepper: its label, value text, and arrows.
type moneyRow struct {
	name     string
	value    func() string
	down, up *ui.Button
}

// cashStep is the step for a sum of money near v: $10k below $100k,
// $50k below $1M, $250k above.
func cashStep(v int) int {
	switch a := max(v, -v); {
	case a < 100_000:
		return 10_000
	case a < 1_000_000:
		return 50_000
	}
	return 250_000
}

func newMoneyTab(m scenarioMoney) *moneyTab {
	t := &moneyTab{m: m}
	// money steps a dollar field by cashStep, between lo and hi.
	money := func(v *int, lo, hi int) (func(), func()) {
		return func() { *v = clampInt(*v-cashStep(*v-1), lo, hi) },
			func() { *v = clampInt(*v+cashStep(*v), lo, hi) }
	}
	// price steps a price by d, between 0 and hi.
	price := func(v *int, d, hi int) (func(), func()) {
		return func() { *v = clampInt(*v-d, 0, hi) }, func() { *v = clampInt(*v+d, 0, hi) }
	}
	add := func(name string, value func() string, down, up func()) {
		t.rows = append(t.rows, moneyRow{
			name: name, value: value,
			down: ui.NewButton(0, 0, 26, detailsRowH, "<", down),
			up:   ui.NewButton(0, 0, 26, detailsRowH, ">", up),
		})
	}
	d, u := money(&t.m.Cash, -10_000_000, 50_000_000)
	add("Starting cash", func() string { return formatDollars(t.m.Cash) }, d, u)
	d, u = money(&t.m.CreditLimit, 0, 50_000_000)
	add("Credit line", func() string { return formatDollars(t.m.CreditLimit) }, d, u)
	rate := func(dp float32) func() {
		return func() {
			pct := clampInt(int(t.m.CreditRate*100+0.5)+int(dp), 0, 40)
			t.m.CreditRate = float32(pct) / 100
		}
	}
	add("Interest a year", func() string { return fmt.Sprintf("%.0f%%", t.m.CreditRate*100) }, rate(-1), rate(1))
	d, u = price(&t.m.DayTicket, 5, 500)
	add("Day ticket", func() string { return formatDollars(t.m.DayTicket) }, d, u)
	d, u = price(&t.m.SeasonPass, 25, 5000)
	add("Season pass", func() string { return formatDollars(t.m.SeasonPass) }, d, u)
	d, u = price(&t.m.Park, 5, 200)
	add("Parking, per car", func() string { return formatDollars(t.m.Park) }, d, u)
	return t
}

func (t *moneyTab) buttons() []*ui.Button {
	var out []*ui.Button
	for _, r := range t.rows {
		out = append(out, r.down, r.up)
	}
	return out
}

// moneyValueW is the width of a Money stepper's value.
const moneyValueW = float32(130)

func (t *moneyTab) layout(x, y float32) {
	for i, r := range t.rows {
		ry := y + float32(i)*(detailsRowH+10)
		r.down.X, r.down.Y = x+mixLabelW, ry
		r.up.X, r.up.Y = x+mixLabelW+26+moneyValueW, ry
	}
}

func (t *moneyTab) draw(r *render.Renderer, x float32) {
	label := mgl32.Vec4{0.8, 0.86, 0.95, 1}
	white := mgl32.Vec4{1, 1, 1, 1}
	textOff := (detailsRowH - float32(render.GlyphH)) / 2
	for _, row := range t.rows {
		v := row.value()
		r.Font.DrawText(r, row.name, x, row.down.Y+textOff, label)
		r.Font.DrawText(r, v, row.down.X+26+(moneyValueW-r.Font.TextWidth(v))/2, row.down.Y+textOff, white)
	}
	y := t.rows[len(t.rows)-1].down.Y + detailsRowH + 24
	for _, line := range []string{
		fmt.Sprintf("The player can spend %s at the start: cash plus the credit line.",
			formatDollars(t.m.Cash+t.m.CreditLimit)),
		"Negative cash starts the resort in debt, drawn on the credit line.",
		fmt.Sprintf("Interest is charged monthly; %d days past the credit line is bankruptcy.",
			world.BankruptcyGraceDays),
		"Prices are where they start; the player can change them.",
	} {
		r.Font.DrawText(r, line, x, y, label)
		y += float32(render.GlyphH) + 8
	}
}
