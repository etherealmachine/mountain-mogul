package ui

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
)

// ChartKind selects the renderer used for a Chart's data.
type ChartKind int

const (
	// ChartLine connects each point with a thin line. Best for cumulative
	// or smoothly-varying values like guest count or cash on hand.
	ChartLine ChartKind = iota

	// ChartGroupedBar draws N coloured bars per point side-by-side. Best
	// for "arrivals vs. departures per day" where the two columns invite
	// direct comparison.
	ChartGroupedBar

	// ChartThoughtRank shows the last day's thought distribution as a
	// ranked list: colour swatch + label | fill bar | percentage. Entries
	// are sorted most-common first; zero-count entries are hidden.
	ChartThoughtRank

	// ChartStats shows one or more named scalar values as label + large-number
	// rows. GetData should return a single ChartPoint whose Values slice has
	// one entry per Series.
	ChartStats
)

// ChartSeries describes one named, coloured line/bar in a chart. Lines
// have one Series; grouped bars have ≥ 1.
type ChartSeries struct {
	Name  string
	Color mgl32.Vec4
}

// ChartPoint is one x-axis tick worth of data — a Label (for the axis)
// plus one Value per Series in the parent Chart. Length of Values must
// equal len(Chart.Series).
type ChartPoint struct {
	Day    time.Time
	Values []float64
}

// Chart is one tab in the ChartWindow. The chart owns its appearance
// (title, switcher icon, kind, series colours); the data source is a
// callback so the scene can keep the History on the World and just feed
// the window each frame.
type Chart struct {
	Title       string
	Icon        render.IconName
	Kind        ChartKind
	Series      []ChartSeries
	GetData     func() []ChartPoint
	FormatValue func(float64) string // optional; used by ChartStats to format each value
}

// ChartWindow hosts one or more Charts with an icon tab strip along the
// top to switch between them. It looks and behaves like ui.Window for
// the chrome (title bar, close button, click consumption) but the body
// is a full-bleed chart panel rather than a row-stack.
type ChartWindow struct {
	Title   string
	X, Y    float32
	Visible bool

	charts []Chart
	active int

	width, height float32

	closeBtn *Button
	tabs     []*chartTab
}

// chartTab is one switcher icon at the top of the window.
type chartTab struct {
	icon       render.IconName
	x, y, w, h float32
	hovered    bool
	onClick    func()
}

const (
	chartWindowW = float32(720)
	chartWindowH = float32(460)
	chartTabSize = float32(40) // square tab cells in the strip
	chartBodyPad = float32(20)
)

// chartTitleH matches winTitleH — both depend on the runtime GlyphH var.
var chartTitleH = winTitleH

// NewChartWindow constructs a ChartWindow at the given position. Charts
// is the ordered tab list; the first chart starts active.
func NewChartWindow(title string, x, y float32, charts []Chart) *ChartWindow {
	cw := &ChartWindow{
		Title:  title,
		X:      x,
		Y:      y,
		width:  chartWindowW,
		height: chartWindowH,
		charts: charts,
	}
	cw.closeBtn = NewButton(0, 0, winBtnW, float32(render.GlyphH+4), "x", func() {
		cw.Visible = false
	})
	cw.tabs = make([]*chartTab, len(charts))
	for i := range charts {
		i := i
		cw.tabs[i] = &chartTab{
			icon:    charts[i].Icon,
			onClick: func() { cw.active = i },
		}
	}
	cw.layout()
	return cw
}

// Center repositions the window so it sits in the middle of the screen.
func (cw *ChartWindow) Center(screenW, screenH int) {
	cw.X = float32(screenW)/2 - cw.width/2
	cw.Y = float32(screenH)/2 - cw.height/2
	cw.layout()
}

// ContainsPoint reports whether (x, y) is inside the window. Used by the
// scene's click router to gate world-clicks under the popup.
func (cw *ChartWindow) ContainsPoint(x, y float32) bool {
	return cw.Visible && x >= cw.X && x <= cw.X+cw.width && y >= cw.Y && y <= cw.Y+cw.height
}

// HandleInput dispatches mouse events to the close button and tab strip.
// Left-clicks landing inside the window mark themselves consumed so the
// scene's world-tools don't fire underneath.
func (cw *ChartWindow) HandleInput(inp *engine.Input) {
	if !cw.Visible {
		return
	}
	mx, my := inp.MousePos[0], inp.MousePos[1]
	if inp.LeftClick && cw.ContainsPoint(mx, my) {
		inp.LeftClickConsumed = true
	}
	cw.closeBtn.SetHovered(cw.closeBtn.Contains(mx, my))
	for _, t := range cw.tabs {
		t.hovered = mx >= t.x && mx <= t.x+t.w && my >= t.y && my <= t.y+t.h
	}
	if !inp.LeftClick {
		return
	}
	if cw.closeBtn.Contains(mx, my) {
		cw.closeBtn.Click()
		return
	}
	for _, t := range cw.tabs {
		if mx >= t.x && mx <= t.x+t.w && my >= t.y && my <= t.y+t.h {
			t.onClick()
			return
		}
	}
}

// layout positions the close button and tab strip relative to the
// window origin. Called after every Reposition / Center.
func (cw *ChartWindow) layout() {
	cw.closeBtn.X = cw.X + cw.width - winBtnW - 4
	cw.closeBtn.Y = cw.Y + (chartTitleH-float32(render.GlyphH+4))/2
	// Tab strip sits directly under the title bar, left-aligned with
	// chartBodyPad inset. Each tab is chartTabSize square; gap of 4 px
	// keeps the icons visually separate.
	const gap = float32(4)
	tabY := cw.Y + chartTitleH + 6
	tabX := cw.X + chartBodyPad
	for _, t := range cw.tabs {
		t.x = tabX
		t.y = tabY
		t.w = chartTabSize
		t.h = chartTabSize
		tabX += chartTabSize + gap
	}
}

// Draw renders the window chrome and the active chart body.
func (cw *ChartWindow) Draw(r *render.Renderer) {
	if !cw.Visible {
		return
	}
	cw.layout()

	bgColor := mgl32.Vec4{0.08, 0.10, 0.14, 0.96}
	titleBg := mgl32.Vec4{0.15, 0.20, 0.35, 1.0}
	textColor := mgl32.Vec4{0.9, 0.95, 1.0, 1.0}

	r.DrawColorRect(cw.X, cw.Y, cw.width, cw.height, bgColor)
	r.DrawColorRect(cw.X, cw.Y, cw.width, chartTitleH, titleBg)
	if r.Font != nil {
		titleY := cw.Y + (chartTitleH-float32(render.GlyphH))/2
		r.Font.DrawText(r, cw.Title, cw.X+winPadding, titleY, textColor)
	}
	cw.closeBtn.Draw(r)

	// Tab strip.
	for i, t := range cw.tabs {
		drawChartTab(r, t, i == cw.active)
	}

	if cw.active < 0 || cw.active >= len(cw.charts) {
		return
	}
	c := cw.charts[cw.active]

	// Chart body rect — between tab strip and window bottom.
	tabBottom := chartTitleH + 6 + chartTabSize + 6
	bodyX := cw.X + chartBodyPad
	bodyY := cw.Y + tabBottom
	bodyW := cw.width - 2*chartBodyPad
	bodyH := cw.height - tabBottom - chartBodyPad

	// Subtitle: chart title above the plotting area.
	if r.Font != nil {
		r.Font.DrawText(r, c.Title, bodyX, bodyY-float32(render.GlyphH)-4, textColor)
	}

	var points []ChartPoint
	if c.GetData != nil {
		points = c.GetData()
	}
	switch c.Kind {
	case ChartLine:
		drawLineChart(r, bodyX, bodyY, bodyW, bodyH, c.Series, points)
	case ChartGroupedBar:
		drawGroupedBarChart(r, bodyX, bodyY, bodyW, bodyH, c.Series, points)
	case ChartThoughtRank:
		drawThoughtRankChart(r, bodyX, bodyY, bodyW, bodyH, c.Series, points)
	case ChartStats:
		drawStatsChart(r, bodyX, bodyY, bodyW, bodyH, c.Series, points, c.FormatValue)
	}
}

// drawChartTab renders one switcher button — icon centred on a slot,
// background tint indicates active vs. hovered vs. idle.
func drawChartTab(r *render.Renderer, t *chartTab, active bool) {
	bg := mgl32.Vec4{0.12, 0.15, 0.22, 1.0}
	if active {
		bg = mgl32.Vec4{0.18, 0.42, 0.28, 1.0}
	} else if t.hovered {
		bg = mgl32.Vec4{0.18, 0.20, 0.28, 1.0}
	}
	r.DrawColorRect(t.x, t.y, t.w, t.h, bg)
	const inset = float32(6)
	r.DrawIcon(t.icon, t.x+inset, t.y+inset, t.w-2*inset, mgl32.Vec4{0.95, 0.95, 1.0, 1})
}

// =============================================================================
// Chart renderers
// =============================================================================

// chartChrome holds the colour palette + helpers shared by the line and
// bar renderers. Pulled out so retuning the look only edits one block.
type chartChrome struct {
	axis     mgl32.Vec4
	grid     mgl32.Vec4
	label    mgl32.Vec4
	empty    mgl32.Vec4
	plotArea mgl32.Vec4
}

var defaultChrome = chartChrome{
	axis:     mgl32.Vec4{0.55, 0.62, 0.78, 1},
	grid:     mgl32.Vec4{0.22, 0.26, 0.34, 1},
	label:    mgl32.Vec4{0.78, 0.84, 0.94, 1},
	empty:    mgl32.Vec4{0.65, 0.70, 0.82, 1},
	plotArea: mgl32.Vec4{0.05, 0.07, 0.10, 1},
}

// plotRect carves the legend out of the body rect and returns the
// remaining axis-bearing region. The legend sits at the bottom, full
// width, with one row of (swatch + name) per series.
func plotRect(x, y, w, h float32, series []ChartSeries) (px, py, pw, ph float32, legendY float32) {
	const legendH = float32(22)
	const leftPad = float32(48) // y-axis label area
	const bottomPad = float32(20)
	legendY = y + h - legendH
	px = x + leftPad
	py = y
	pw = w - leftPad
	ph = h - legendH - bottomPad
	return
}

// drawLegend renders a horizontal row of (colour swatch + series name)
// at (x, y). Returns the total drawn width — caller centres it.
func drawLegend(r *render.Renderer, x, y float32, series []ChartSeries) {
	if r.Font == nil {
		return
	}
	const swatch = float32(12)
	const gap = float32(6)
	const itemGap = float32(20)
	cur := x
	textOffY := -float32(2) // raise text so it sits on the same baseline as the swatch
	for _, s := range series {
		r.DrawColorRect(cur, y, swatch, swatch, s.Color)
		cur += swatch + gap
		r.Font.DrawText(r, s.Name, cur, y+textOffY, defaultChrome.label)
		cur += float32(len(s.Name)*render.GlyphAdvance) + itemGap
	}
}

// rangeOf returns the min/max value across every series of every point,
// with a small head-room pad. Empty input → (0, 1).
func rangeOf(points []ChartPoint) (lo, hi float64) {
	if len(points) == 0 {
		return 0, 1
	}
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, p := range points {
		for _, v := range p.Values {
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	if math.IsInf(lo, 0) || math.IsInf(hi, 0) {
		return 0, 1
	}
	// 5 % head-room above; floor at 0 below if the series is non-negative.
	span := hi - lo
	if span < 1 {
		span = 1
	}
	hi += span * 0.05
	if lo >= 0 {
		lo = 0
	} else {
		lo -= span * 0.05
	}
	return
}

// drawAxes renders the left + bottom axis lines and a few horizontal
// gridlines. Returns nothing — callers know the plot rect themselves.
func drawAxes(r *render.Renderer, px, py, pw, ph float32, lo, hi float64) {
	chrome := defaultChrome
	r.DrawColorLine(px, py+ph, px+pw, py+ph, 1, chrome.axis)
	r.DrawColorLine(px, py, px, py+ph, 1, chrome.axis)
	// Five horizontal gridlines + their y-axis labels.
	if r.Font == nil {
		return
	}
	const ticks = 5
	for i := 1; i <= ticks; i++ {
		frac := float32(i) / float32(ticks)
		gy := py + ph - ph*frac
		r.DrawColorLine(px, gy, px+pw, gy, 1, chrome.grid)
		v := lo + (hi-lo)*float64(frac)
		label := formatTick(v)
		labelW := r.Font.TextWidth(label)
		r.Font.DrawText(r, label, px-labelW-6, gy-float32(render.GlyphH)/2, chrome.label)
	}
	zero := formatTick(lo)
	zeroW := r.Font.TextWidth(zero)
	r.Font.DrawText(r, zero, px-zeroW-6, py+ph-float32(render.GlyphH)/2, chrome.label)
}

// drawXLabels stamps a start / mid / end date label under the x-axis.
func drawXLabels(r *render.Renderer, px, py, pw, ph float32, points []ChartPoint) {
	if r.Font == nil || len(points) == 0 {
		return
	}
	chrome := defaultChrome
	y := py + ph + 4
	stamp := func(i int, anchor float32) {
		label := points[i].Day.Format("Jan 2")
		w := r.Font.TextWidth(label)
		r.Font.DrawText(r, label, anchor-w/2, y, chrome.label)
	}
	stamp(0, px)
	if len(points) > 2 {
		stamp(len(points)/2, px+pw/2)
	}
	stamp(len(points)-1, px+pw)
}

// drawNoData centres a placeholder message in the plot area.
func drawNoData(r *render.Renderer, px, py, pw, ph float32) {
	if r.Font == nil {
		return
	}
	msg := "No data yet — come back after a few days"
	w := r.Font.TextWidth(msg)
	r.Font.DrawText(r, msg, px+(pw-w)/2, py+ph/2-float32(render.GlyphH)/2, defaultChrome.empty)
}

func drawLineChart(r *render.Renderer, x, y, w, h float32, series []ChartSeries, points []ChartPoint) {
	px, py, pw, ph, legendY := plotRect(x, y, w, h, series)
	r.DrawColorRect(px, py, pw, ph, defaultChrome.plotArea)

	if len(points) == 0 {
		drawNoData(r, px, py, pw, ph)
		drawLegend(r, px, legendY+4, series)
		return
	}
	lo, hi := rangeOf(points)
	drawAxes(r, px, py, pw, ph, lo, hi)
	drawXLabels(r, px, py, pw, ph, points)

	// Plot every series. Lines are 2 px thick; with the white fallback
	// texture this reads cleanly against the dark plot background.
	if len(points) >= 1 {
		denom := float32(len(points) - 1)
		if denom < 1 {
			denom = 1
		}
		for si, s := range series {
			if si >= len(points[0].Values) {
				continue
			}
			var prevX, prevY float32
			for i, p := range points {
				if si >= len(p.Values) {
					continue
				}
				v := p.Values[si]
				xPos := px + (float32(i)/denom)*pw
				yPos := py + ph - float32((v-lo)/(hi-lo))*ph
				if i > 0 {
					r.DrawColorLine(prevX, prevY, xPos, yPos, 2, s.Color)
				}
				prevX, prevY = xPos, yPos
			}
		}
	}
	drawLegend(r, px, legendY+4, series)
}

func drawGroupedBarChart(r *render.Renderer, x, y, w, h float32, series []ChartSeries, points []ChartPoint) {
	px, py, pw, ph, legendY := plotRect(x, y, w, h, series)
	r.DrawColorRect(px, py, pw, ph, defaultChrome.plotArea)

	if len(points) == 0 {
		drawNoData(r, px, py, pw, ph)
		drawLegend(r, px, legendY+4, series)
		return
	}
	lo, hi := rangeOf(points)
	drawAxes(r, px, py, pw, ph, lo, hi)
	drawXLabels(r, px, py, pw, ph, points)

	// Bar width: each "group" gets `pw / N` of horizontal space; inside
	// the group each series gets equal share with a small gap.
	n := float32(len(points))
	groupW := pw / n
	const groupPad = float32(0.18) // 18 % of the group is empty so bars don't kiss
	innerW := groupW * (1 - groupPad)
	barW := innerW / float32(len(series))
	if barW < 1 {
		barW = 1
	}
	for i, p := range points {
		groupX := px + float32(i)*groupW + (groupW-innerW)/2
		for si, s := range series {
			if si >= len(p.Values) {
				continue
			}
			v := p.Values[si]
			if v <= lo {
				continue
			}
			bh := float32((v-lo)/(hi-lo)) * ph
			bx := groupX + float32(si)*barW
			r.DrawColorRect(bx, py+ph-bh, barW, bh, s.Color)
		}
	}
	drawLegend(r, px, legendY+4, series)
}

func drawThoughtRankChart(r *render.Renderer, x, y, w, h float32, series []ChartSeries, points []ChartPoint) {
	if len(points) == 0 {
		drawNoData(r, x, y, w, h)
		return
	}
	last := points[len(points)-1]

	type entry struct {
		name  string
		color mgl32.Vec4
		count float64
	}
	entries := make([]entry, 0, len(series))
	total := 0.0
	for i, s := range series {
		v := 0.0
		if i < len(last.Values) {
			v = last.Values[i]
		}
		total += v
		entries = append(entries, entry{s.Name, s.Color, v})
	}
	if total == 0 {
		drawNoData(r, x, y, w, h)
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].count > entries[j].count })

	// Optional date label top-right.
	if r.Font != nil && !last.Day.IsZero() {
		dateStr := "as of " + last.Day.Format("Jan 2")
		dw := r.Font.TextWidth(dateStr)
		r.Font.DrawText(r, dateStr, x+w-dw, y, defaultChrome.label)
	}

	const (
		swatchSz  = float32(10)
		swatchGap = float32(10)
		rowH      = float32(22)
		rowGap    = float32(8)
		pctColW   = float32(48) // "100%" + margin
		barGap    = float32(14)
	)

	// Label column width: longest name.
	maxChars := 0
	for _, e := range entries {
		if len(e.name) > maxChars {
			maxChars = len(e.name)
		}
	}
	labelColW := float32(maxChars*render.GlyphAdvance) + 4

	barX := x + swatchSz + swatchGap + labelColW + barGap
	barMaxW := w - (swatchSz + swatchGap + labelColW + barGap + barGap + pctColW)
	if barMaxW < 0 {
		barMaxW = 0
	}

	// Leave a gap below the date label.
	startY := y + float32(render.GlyphH) + 10

	row := 0
	for _, e := range entries {
		if e.count == 0 {
			continue
		}
		ry := startY + float32(row)*(rowH+rowGap)
		pct := e.count / total

		// Swatch.
		r.DrawColorRect(x, ry+(rowH-swatchSz)/2, swatchSz, swatchSz, e.color)

		// Label.
		if r.Font != nil {
			r.Font.DrawText(r, e.name, x+swatchSz+swatchGap, ry+(rowH-float32(render.GlyphH))/2, defaultChrome.label)
		}

		// Bar track + fill.
		trackColor := mgl32.Vec4{e.color[0] * 0.20, e.color[1] * 0.20, e.color[2] * 0.20, 0.6}
		barY := ry + (rowH-10)/2
		r.DrawColorRect(barX, barY, barMaxW, 10, trackColor)
		bw := float32(pct) * barMaxW
		r.DrawColorRect(barX, barY, bw, 10, e.color)

		// Percentage.
		if r.Font != nil {
			pctStr := fmt.Sprintf("%.0f%%", pct*100)
			r.Font.DrawText(r, pctStr, barX+barMaxW+barGap, ry+(rowH-float32(render.GlyphH))/2, defaultChrome.label)
		}

		row++
	}
}

func drawStatsChart(r *render.Renderer, x, y, w, h float32, series []ChartSeries, points []ChartPoint, formatValue func(float64) string) {
	if r.Font == nil {
		return
	}
	var vals []float64
	if len(points) > 0 {
		vals = points[0].Values
	}

	format := formatValue
	if format == nil {
		format = func(v float64) string { return fmt.Sprintf("%.0f", v) }
	}

	const rowH = float32(52)
	labelColor := defaultChrome.label

	for i, s := range series {
		v := 0.0
		if i < len(vals) {
			v = vals[i]
		}
		ry := y + float32(i)*rowH
		r.Font.DrawText(r, s.Name, x, ry, labelColor)
		r.Font.DrawText(r, format(v), x, ry+float32(render.GlyphH)+6, s.Color)
	}
}

// formatTick renders a y-axis tick value compactly. Large numbers get
// k/M suffixes so the axis column stays narrow.
func formatTick(v float64) string {
	abs := math.Abs(v)
	switch {
	case abs >= 1e6:
		return fmt.Sprintf("%.1fM", v/1e6)
	case abs >= 1e3:
		return fmt.Sprintf("%.1fk", v/1e3)
	case abs == math.Trunc(abs):
		return fmt.Sprintf("%.0f", v)
	default:
		return fmt.Sprintf("%.1f", v)
	}
}
