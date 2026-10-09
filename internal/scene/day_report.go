package scene

import (
	"fmt"
	"strconv"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/settings"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

var (
	reportText   = mgl32.Vec4{0.9, 0.95, 1.0, 1}
	reportDim    = mgl32.Vec4{0.6, 0.66, 0.75, 1}
	reportProfit = mgl32.Vec4{0.45, 0.9, 0.5, 1}
	reportLoss   = mgl32.Vec4{1.0, 0.45, 0.4, 1}
)

// onDayRollover opens the profit/loss report for the day just closed,
// replacing any earlier one still on screen (turbo can cross several
// midnights between frames; only the latest is shown).
func (s *Scenario) onDayRollover() {
	if settings.Get().HideDailyReport || s.world == nil || s.world.History == nil {
		return
	}
	samples := s.world.History.Ordered()
	if len(samples) == 0 {
		return
	}
	var prev *world.DailySample
	if len(samples) > 1 {
		prev = &samples[len(samples)-2]
	}
	s.openDayReport(samples[len(samples)-1], prev)
}

// openDayReport builds the report window for sample d. prev is the day
// before (nil for the first recorded day); it separates construction and
// purchases from the operating result.
func (s *Scenario) openDayReport(d world.DailySample, prev *world.DailySample) {
	title := world.FormatGameDate(d.Day, false)
	if _, name := world.HolidayAt(d.Day); name != "" {
		title += " (" + name + ")"
	}
	win := ui.NewWindow(title+" Report", 0, 0)
	if !d.Open {
		win.AddAmount("Resort", "Closed all day", reportDim, false)
	}

	win.AddSection("Revenue")
	for k := world.RevenueKind(0); k < world.RevenueKindCount; k++ {
		if v := d.RevenueByKind[k]; v != 0 || k == world.RevenueDayTickets {
			win.AddAmount(k.Label(), formatDollars(v), reportText, false)
		}
	}
	win.AddAmount("Total revenue", formatDollars(d.Revenue), reportText, true)

	win.AddSection("Costs")
	for k := world.CostKind(0); k < world.CostKindCount; k++ {
		if v := d.CostsByKind[k]; v != 0 {
			win.AddAmount(k.Label(), formatDollars(v), reportText, false)
		}
	}
	win.AddAmount("Total costs", formatDollars(d.Costs), reportText, true)

	net := d.Revenue - d.Costs
	win.AddSection("Result")
	if net >= 0 {
		win.AddAmount("Profit", "+"+formatDollars(net), reportProfit, false)
	} else {
		win.AddAmount("Loss", formatDollars(net), reportLoss, false)
	}
	if prev != nil {
		if capital := prev.Cash + net - d.Cash; capital != 0 {
			win.AddAmount("Capital spending", formatDollars(capital), reportDim, false)
		}
	}
	cashColor := reportText
	if d.Cash < 0 {
		cashColor = reportLoss
	}
	win.AddAmount("Cash", formatDollars(d.Cash), cashColor, false)
	win.AddAmount("Visitors", strconv.Itoa(d.ArrivalsToday), reportText, false)

	win.AddBoolToggle("Show every night",
		func() bool { return !settings.Get().HideDailyReport },
		func(on bool) {
			settings.Get().HideDailyReport = !on
			_ = settings.Save()
		})
	win.AddActionButton("Open charts", func() {
		win.Visible = false
		s.chartWindow.Visible = true
		s.topBar.SetChartsActive(true)
	})

	win.Visible = true
	r := s.app.Renderer
	win.Center(r.ScreenWidth(), r.ScreenHeight())
	s.dayReport = win
}

// formatDollars renders n as "$12,345" or "-$12,345".
func formatDollars(n int) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	digits := strconv.Itoa(n)
	out := make([]byte, 0, len(digits)+len(digits)/3)
	for i := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, digits[i])
	}
	return fmt.Sprintf("%s$%s", sign, out)
}
