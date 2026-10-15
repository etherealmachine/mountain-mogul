package scene

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// SaveList is the games list (Saved Games): every game in
// ~/.mountain-mogul/saves/, most recently played first. A row continues
// its game; Rename, Copy and Delete sit beside it. Back returns to the
// start menu.
type SaveList struct {
	app     *engine.App
	rows    []*gameRow
	back    *ui.Button
	empty   bool
	scroll  float32
	prompt  *savePrompt    // renaming a game
	confirm *confirmPrompt // deleting one
	message string         // a rename refused, shown under the title
}

// gameRow is one game: the row that continues it and its three actions.
type gameRow struct {
	game                *save.GameInfo
	play                *ui.Button
	rename, copy, trash *ui.Button
}

func NewSaveList() *SaveList { return &SaveList{} }

const (
	gameRowW, gameRowH, gameRowGap = float32(640), float32(40), float32(10)
	gameActW, gameActGap           = float32(74), float32(6)
)

func (s *SaveList) Init(app *engine.App) error {
	s.app = app
	s.back = ui.NewButton(24, 24, 120, 40, "Back", func() { s.app.PopScene() })
	s.back.Color = mgl32.Vec4{0.4, 0.25, 0.25, 0.95}
	s.back.HoverColor = mgl32.Vec4{0.6, 0.4, 0.4, 0.95}
	s.reload()
	return nil
}

// reload reads the games list again, after a rename, copy or delete.
func (s *SaveList) reload() {
	games := save.ListGames()
	s.empty = len(games) == 0
	s.rows = s.rows[:0]
	now := time.Now()
	for i := range games {
		g := &games[i]
		row := &gameRow{game: g}
		path := g.Path
		row.play = ui.NewButton(0, 0, 0, gameRowH, gameLabel(g, now), func() {
			s.app.ReplaceScene(NewScenarioFromFile(path))
		})
		row.play.Color = mgl32.Vec4{0.15, 0.25, 0.45, 0.95}
		row.play.HoverColor = mgl32.Vec4{0.25, 0.45, 0.75, 0.95}
		row.rename = ui.NewButton(0, 0, gameActW, gameRowH, "Rename", func() { s.openRename(g) })
		row.copy = ui.NewButton(0, 0, gameActW, gameRowH, "Copy", func() {
			if _, err := save.CopyGame(g.Name); err != nil {
				s.message = "Couldn't copy: " + err.Error()
			}
			s.reload()
		})
		row.trash = ui.NewButton(0, 0, gameActW, gameRowH, "Delete", func() {
			s.confirm = newConfirmPrompt("Delete "+g.Name+"? This can't be undone.", "Delete",
				func() {
					s.confirm = nil
					if err := save.DeleteGame(g.Name); err != nil {
						s.message = "Couldn't delete: " + err.Error()
					}
					s.reload()
				},
				func() { s.confirm = nil })
		})
		s.rows = append(s.rows, row)
	}
}

// openRename asks for g's new name; a name another game has is refused
// and the prompt stays up.
func (s *SaveList) openRename(g *save.GameInfo) {
	s.prompt = newSavePrompt(g.Name,
		func(name string) {
			if err := save.RenameGame(g.Name, name); err != nil {
				if errors.Is(err, save.ErrGameExists) {
					s.message = "There's already a game called " + save.SanitizeSaveName(name)
				} else {
					s.message = "Couldn't rename: " + err.Error()
				}
				return
			}
			s.prompt, s.message = nil, ""
			s.reload()
		},
		func() { s.prompt, s.message = nil, "" },
	)
	s.prompt.title = "Rename " + g.Name
	s.prompt.okBtn.Label = "Rename"
}

// gameLabel is a game's row: its name, its scenario when the name isn't
// the scenario's, the date in the game, the rating, won or lost, and when
// it was last played.
func gameLabel(g *save.GameInfo, now time.Time) string {
	parts := []string{g.Name}
	if sc := g.Scenario.Name; sc != "" && !strings.HasPrefix(g.Name, sc) {
		parts[0] += " (" + sc + ")"
	}
	if !g.Date.IsZero() {
		parts = append(parts, world.FormatGameDate(g.Date, false), fmt.Sprintf("%.1f stars", g.Stars))
	}
	switch g.Outcome {
	case world.Won:
		parts = append(parts, "won")
	case world.Lost:
		parts = append(parts, "lost")
	}
	parts = append(parts, relativeTime(now, g.ModTime))
	return strings.Join(parts, "  ·  ")
}

func (s *SaveList) layout() {
	sw := float32(s.app.Renderer.ScreenWidth())
	sh := float32(s.app.Renderer.ScreenHeight())
	total := float32(len(s.rows)) * (gameRowH + gameRowGap)
	// Centred when it fits; else it starts under the title and scrolls.
	room := sh - 140 - 20
	s.scroll = max(0, min(s.scroll, total-room))
	y := max((sh-total)/2, 140) - s.scroll
	x := (sw - gameRowW) / 2
	playW := gameRowW - 3*(gameActW+gameActGap)
	for _, row := range s.rows {
		row.play.X, row.play.Y, row.play.W = x, y, playW
		ax := x + playW + gameActGap
		for _, b := range []*ui.Button{row.rename, row.copy, row.trash} {
			b.X, b.Y = ax, y
			ax += gameActW + gameActGap
		}
		y += gameRowH + gameRowGap
	}
}

// listTop is the top of the first row, or the middle when there are none.
func (s *SaveList) listTop() float32 {
	if len(s.rows) > 0 {
		return s.rows[0].play.Y
	}
	return float32(s.app.Renderer.ScreenHeight()) / 2
}

func (s *SaveList) Update(dt float64) {
	tickMenuBackdrop(s.app, dt)
	inp := s.app.Input
	sw := float32(s.app.Renderer.ScreenWidth())
	sh := float32(s.app.Renderer.ScreenHeight())
	if s.confirm != nil {
		s.confirm.HandleInput(inp, sw, sh)
		return
	}
	if s.prompt != nil {
		s.prompt.HandleInput(inp, sw, sh)
		return
	}
	if inp.Pressed[glfw.KeyEscape] {
		s.app.PopScene()
		return
	}
	s.scroll -= inp.ScrollDelta * 40
	s.layout()
	mx, my := inp.MousePos[0], inp.MousePos[1]
	buttons := []*ui.Button{s.back}
	for _, row := range s.rows {
		buttons = append(buttons, row.play, row.rename, row.copy, row.trash)
	}
	for _, btn := range buttons {
		btn.SetHovered(btn.Contains(mx, my))
		if inp.LeftClick && btn.Contains(mx, my) {
			btn.Click()
			return
		}
	}
}

func (s *SaveList) Render(r *render.Renderer) {
	gl.ClearColor(0.635, 0.682, 0.918, 1.0)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
	renderMenuBackdrop(r)

	sw := float32(r.ScreenWidth())
	s.layout()
	top := s.listTop()
	drawables := []render.UIDrawable{&titleDrawable{x: sw/2 - 100, y: top - 90, w: 200, h: 40}}
	if s.empty {
		drawables = append(drawables, &emptyHint{
			text: "No games yet — click New Game to start one.",
			y:    top,
		})
	}
	if s.message != "" && s.prompt == nil {
		drawables = append(drawables, &emptyHint{text: s.message, y: top - 30})
	}
	for _, row := range s.rows {
		drawables = append(drawables, row.play, row.rename, row.copy, row.trash)
	}
	drawables = append(drawables, s.back)
	if s.prompt != nil {
		drawables = append(drawables, s.prompt)
		if s.message != "" {
			drawables = append(drawables, &emptyHint{text: s.message, y: float32(r.ScreenHeight())/2 + savePromptH/2 + 12})
		}
	}
	if s.confirm != nil {
		drawables = append(drawables, s.confirm)
	}
	r.DrawUI(drawables)
}

func (s *SaveList) Destroy() {}

// emptyHint draws a single line of dimmed text at the given Y, horizontally
// centred. Used when the SaveList is empty.
type emptyHint struct {
	text string
	y    float32
}

func (e *emptyHint) Draw(r *render.Renderer) {
	if r.Font == nil {
		return
	}
	w := r.Font.TextWidth(e.text)
	x := (float32(r.ScreenWidth()) - w) / 2
	r.Font.DrawText(r, e.text, x, e.y, mgl32.Vec4{0.85, 0.85, 0.85, 1})
}

// relativeTime renders a coarse "5m ago" / "2h ago" / "3d ago" string for
// the SaveList rows. Avoids importing humanize for one helper.
func relativeTime(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
