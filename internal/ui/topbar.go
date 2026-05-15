package ui

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
)

// TopBar is the resort-management HUD strip across the top of the screen.
// Three regions: stats (left), date+weather (centre), speed/settings
// (right). Layout is recomputed from the screen width every frame so resize
// works without re-registering anything.
type TopBar struct {
	Y, H float32

	// Stats — pure read-only callbacks. The bar pulls fresh values every
	// frame so the simulation can mutate the underlying state freely.
	GetCash      func() int
	GetGuests    func() int
	GetHappiness func() float32 // 0..1

	// Date + weather snapshot. Same callback pattern as stats so the bar
	// doesn't hold any simulation state of its own.
	GetDate    func() (day int, month string, year int)
	GetWeather func() (now, next WeatherKind, tempF int)

	// Speed-control buttons — index-aligned with SpeedOptions in the
	// scenario. The scenario sets active state through SetSpeedActive /
	// SetPauseActive after each click.
	pauseBtn   *iconButton
	speedBtns  []*iconButton
	gearBtn    *iconButton
	overlayBtn *iconButton // overlay-panel toggle; sits between speed and gear
	chartsBtn  *iconButton // chart-window toggle; sits next to overlay

	bgColor mgl32.Vec4
}

// NewTopBar creates a top bar at (y=0, h=h). The bar fills the full screen
// width every frame.
func NewTopBar(h float32) *TopBar {
	return &TopBar{
		Y:       0,
		H:       h,
		bgColor: mgl32.Vec4{0.06, 0.08, 0.12, 0.96},
	}
}

// SetOverlayToggle installs the overlay-panel toggle button. The button
// sits to the left of the gear button and tracks the panel's visible
// state via SetOverlayActive.
func (t *TopBar) SetOverlayToggle(onClick func()) {
	t.overlayBtn = newIconButton("overlay", onClick)
}

// SetOverlayActive reflects the panel's current visibility back to the
// top-bar icon so it reads as toggled-on while the panel is open.
func (t *TopBar) SetOverlayActive(active bool) {
	if t.overlayBtn != nil {
		t.overlayBtn.active = active
	}
}

// SetChartsToggle installs the charts-window toggle button. Sits left
// of the overlay button; tracks visibility via SetChartsActive.
func (t *TopBar) SetChartsToggle(onClick func()) {
	t.chartsBtn = newIconButton("charts", onClick)
}

// SetChartsActive reflects the chart window's visibility back to the
// top-bar icon.
func (t *TopBar) SetChartsActive(active bool) {
	if t.chartsBtn != nil {
		t.chartsBtn.active = active
	}
}

// SetSettingsButton installs the gear (settings) button at the far right.
// Scenes that don't want speed controls can call this alone.
func (t *TopBar) SetSettingsButton(onGear func()) {
	t.gearBtn = newIconButton("gear", onGear)
}

// SetSpeedControls installs the pause + speed-preset buttons. The
// caller passes one callback per speedOptions entry; the visual
// glyphs (play / fast-forward / double-fast-forward) are decoupled
// from the underlying multipliers so retuning the speeds doesn't
// require new icons.
func (t *TopBar) SetSpeedControls(onPause func(), onSpeed []func()) {
	t.pauseBtn = newIconButton("pause", onPause)
	kinds := []string{"play", "ff2", "ff4"}
	if len(onSpeed) != len(kinds) {
		// Be defensive: stick to as many slots as we have callbacks. The
		// scenario today always passes exactly three.
		if len(onSpeed) < len(kinds) {
			kinds = kinds[:len(onSpeed)]
		}
	}
	t.speedBtns = make([]*iconButton, len(onSpeed))
	for i := range onSpeed {
		t.speedBtns[i] = newIconButton(kinds[i], onSpeed[i])
	}
}

// SetSpeedActive marks the speed button at index i as active and clears the
// rest, plus the pause button. Pass -1 to clear all (useful when paused).
func (t *TopBar) SetSpeedActive(i int) {
	for idx, b := range t.speedBtns {
		b.active = idx == i
	}
	if t.pauseBtn != nil {
		t.pauseBtn.active = false
	}
}

// SetPauseActive marks the pause button as active and clears speed buttons.
func (t *TopBar) SetPauseActive(active bool) {
	if t.pauseBtn != nil {
		t.pauseBtn.active = active
	}
	if active {
		for _, b := range t.speedBtns {
			b.active = false
		}
	}
}

// HandleInput processes mouse input for the bar's clickable regions.
func (t *TopBar) HandleInput(input *engine.Input, screenW float32) {
	t.layout(screenW)
	mx := input.MousePos[0]
	my := input.MousePos[1]
	for _, b := range t.iconButtons() {
		b.hovered = b.contains(mx, my)
		if input.LeftClick && b.contains(mx, my) {
			input.LeftClickConsumed = true
			if b.onClick != nil {
				b.onClick()
			}
		}
	}
}

// ContainsY reports whether y is inside the bar — used so world clicks
// don't fire when clicking on bar real estate.
func (t *TopBar) ContainsY(y float32) bool {
	return y >= t.Y && y <= t.Y+t.H
}

// Draw renders the bar background and all three regions.
func (t *TopBar) Draw(r *render.Renderer) {
	screenW := float32(r.ScreenWidth())
	t.layout(screenW)

	r.DrawColorRect(0, t.Y, screenW, t.H, t.bgColor)

	t.drawStats(r)
	t.drawCenter(r, screenW)
	t.drawRight(r)
}

// iconButtons returns every clickable icon for hover/click iteration.
func (t *TopBar) iconButtons() []*iconButton {
	out := make([]*iconButton, 0, len(t.speedBtns)+4)
	if t.pauseBtn != nil {
		out = append(out, t.pauseBtn)
	}
	out = append(out, t.speedBtns...)
	if t.chartsBtn != nil {
		out = append(out, t.chartsBtn)
	}
	if t.overlayBtn != nil {
		out = append(out, t.overlayBtn)
	}
	if t.gearBtn != nil {
		out = append(out, t.gearBtn)
	}
	return out
}

// layout positions the right-edge icon buttons. Stats and centre are
// laid out at draw time directly.
func (t *TopBar) layout(screenW float32) {
	const pad = float32(8)
	const iconBoxW = float32(48) // icon + label column
	x := screenW - pad

	if t.gearBtn != nil {
		x -= iconBoxW
		t.gearBtn.x = x
		t.gearBtn.y = t.Y
		t.gearBtn.w = iconBoxW
		t.gearBtn.h = t.H
	}
	if t.overlayBtn != nil {
		x -= iconBoxW
		t.overlayBtn.x = x
		t.overlayBtn.y = t.Y
		t.overlayBtn.w = iconBoxW
		t.overlayBtn.h = t.H
	}
	if t.chartsBtn != nil {
		x -= iconBoxW
		t.chartsBtn.x = x
		t.chartsBtn.y = t.Y
		t.chartsBtn.w = iconBoxW
		t.chartsBtn.h = t.H
	}
	for i := len(t.speedBtns) - 1; i >= 0; i-- {
		x -= iconBoxW
		t.speedBtns[i].x = x
		t.speedBtns[i].y = t.Y
		t.speedBtns[i].w = iconBoxW
		t.speedBtns[i].h = t.H
	}
	if t.pauseBtn != nil {
		x -= iconBoxW
		t.pauseBtn.x = x
		t.pauseBtn.y = t.Y
		t.pauseBtn.w = iconBoxW
		t.pauseBtn.h = t.H
	}
}

// drawStats renders cash / guests / happiness vertically stacked on the
// left. Each row is icon + value text.
func (t *TopBar) drawStats(r *render.Renderer) {
	if r.Font == nil {
		return
	}
	const iconSize = float32(20)
	const pad = float32(10)
	col := mgl32.Vec4{0.95, 0.95, 1.0, 1}

	// Three rows fit inside H if H >= 3*(iconSize+gap).
	rowH := t.H / 3
	textY := func(rowIdx int) float32 {
		return t.Y + float32(rowIdx)*rowH + (rowH-float32(render.GlyphH))/2
	}
	iconY := func(rowIdx int) float32 {
		return t.Y + float32(rowIdx)*rowH + (rowH-iconSize)/2
	}

	// Row 0: Cash
	if t.GetCash != nil {
		IconCoin(r, pad, iconY(0), iconSize, mgl32.Vec4{1, 0.85, 0.25, 1})
		text := fmt.Sprintf("$%d", t.GetCash())
		r.Font.DrawText(r, text, pad+iconSize+8, textY(0), col)
	}

	// Row 1: Guests
	if t.GetGuests != nil {
		IconPerson(r, pad, iconY(1), iconSize, mgl32.Vec4{0.55, 0.85, 1.0, 1})
		text := fmt.Sprintf("%d", t.GetGuests())
		r.Font.DrawText(r, text, pad+iconSize+8, textY(1), col)
	}

	// Row 2: Happiness — heart icon + a slim filled bar.
	if t.GetHappiness != nil {
		IconHeart(r, pad, iconY(2), iconSize, mgl32.Vec4{0.95, 0.45, 0.55, 1})
		barX := pad + iconSize + 8
		barY := t.Y + float32(2)*rowH + (rowH-8)/2
		barW := float32(80)
		barH := float32(8)
		r.DrawColorRect(barX, barY, barW, barH, mgl32.Vec4{0.20, 0.22, 0.28, 1})
		h := t.GetHappiness()
		if h < 0 {
			h = 0
		}
		if h > 1 {
			h = 1
		}
		fill := mgl32.Vec4{0.30, 0.85, 0.45, 1}
		if h < 0.5 {
			fill = mgl32.Vec4{0.95, 0.65, 0.25, 1}
		}
		if h < 0.25 {
			fill = mgl32.Vec4{0.90, 0.30, 0.30, 1}
		}
		r.DrawColorRect(barX, barY, barW*h, barH, fill)
		// Numeric % to the right of the bar.
		text := fmt.Sprintf("%d%%", int(h*100))
		r.Font.DrawText(r, text, barX+barW+8, textY(2), col)
	}
}

// drawCenter renders the date stacked above the weather strip in the
// horizontal middle of the bar.
func (t *TopBar) drawCenter(r *render.Renderer, screenW float32) {
	if r.Font == nil {
		return
	}

	col := mgl32.Vec4{0.95, 0.95, 1.0, 1}
	dim := mgl32.Vec4{0.78, 0.82, 0.92, 1}

	// Date line.
	if t.GetDate != nil {
		day, month, year := t.GetDate()
		dateText := fmt.Sprintf("%s %d, Year %d", month, day, year)
		dateW := float32(len(dateText) * render.GlyphAdvance)
		dateX := (screenW - dateW) / 2
		dateY := t.Y + t.H*0.18
		r.Font.DrawText(r, dateText, dateX, dateY, col)
	}

	// Weather row: temp text   [iconNow]  →  [iconNext]
	if t.GetWeather == nil {
		return
	}
	const iconSize = float32(22)
	now, next, tempF := t.GetWeather()
	tempText := fmt.Sprintf("%d F", tempF)
	tempW := float32(len(tempText) * render.GlyphAdvance)
	gap := float32(8)
	rowW := tempW + gap + iconSize + gap + iconSize + gap + iconSize
	rowX := (screenW - rowW) / 2
	rowY := t.Y + t.H*0.55
	r.Font.DrawText(r, tempText, rowX, rowY+(iconSize-float32(render.GlyphH))/2, dim)
	cur := rowX + tempW + gap
	DrawWeatherIcon(r, now, cur, rowY, iconSize)
	cur += iconSize + gap
	IconArrow(r, cur, rowY, iconSize, dim)
	cur += iconSize + gap
	DrawWeatherIcon(r, next, cur, rowY, iconSize)
}

// drawRight renders the speed/pause/gear icon row.
func (t *TopBar) drawRight(r *render.Renderer) {
	for _, b := range t.iconButtons() {
		t.drawIconButton(r, b)
	}
}

// drawIconButton renders one icon button: hover/active background, then
// the icon glyph centred in the box. No text labels — the icon shape
// (pause / play / one fast-forward / two fast-forwards / gear) carries
// the meaning.
func (t *TopBar) drawIconButton(r *render.Renderer, b *iconButton) {
	// Background — subtle highlight on hover, brighter on active.
	switch {
	case b.active:
		r.DrawColorRect(b.x, b.y, b.w, b.h, mgl32.Vec4{0.18, 0.42, 0.28, 1})
	case b.hovered:
		r.DrawColorRect(b.x, b.y, b.w, b.h, mgl32.Vec4{0.18, 0.20, 0.28, 1})
	}

	const iconSize = float32(28)
	col := mgl32.Vec4{0.95, 0.95, 1.0, 1}

	cy := b.y + (b.h-iconSize)/2
	cx := b.x + b.w/2

	switch b.kind {
	case "pause":
		IconPause(r, cx-iconSize/2, cy, iconSize, col)
	case "play":
		IconPlay(r, cx-iconSize/2, cy, iconSize, col)
	case "ff2":
		IconFastForward(r, cx-iconSize/2, cy, iconSize, col)
	case "ff4":
		// 4× = two fast-forwards side-by-side. Each half-icon is narrower
		// than the 2× glyph so the pair fits the same tile width without
		// clipping; reads as "extra fast" at a glance.
		half := iconSize * 0.85
		gap := float32(2)
		totalW := half*2 + gap
		left := cx - totalW/2
		IconFastForward(r, left, cy, half, col)
		IconFastForward(r, left+half+gap, cy, half, col)
	case "gear":
		IconGear(r, cx-iconSize/2, cy, iconSize, col)
	case "overlay":
		r.DrawIcon(render.IconStack, cx-iconSize/2, cy, iconSize, col)
	case "charts":
		r.DrawIcon(render.IconChartBar, cx-iconSize/2, cy, iconSize, col)
	}
}

// iconButton is the TopBar's lightweight clickable region. It exists
// alongside ui.Button (which is text-labelled) because the hover / draw
// paths here are different enough that subclassing ui.Button would be more
// confusing than a parallel type.
type iconButton struct {
	kind          string // "pause", "play", "ff2", "ff4", "gear"
	x, y, w, h    float32
	hovered       bool
	active        bool
	onClick       func()
}

func newIconButton(kind string, onClick func()) *iconButton {
	return &iconButton{kind: kind, onClick: onClick}
}

func (b *iconButton) contains(mx, my float32) bool {
	return mx >= b.x && mx <= b.x+b.w && my >= b.y && my <= b.y+b.h
}
