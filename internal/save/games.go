package save

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mountain-mogul/internal/world"
)

// A game is one save file in SavesDir, named by the game's name: "Boreal",
// "Boreal 2". It's saved as it goes (on starting, each night, on
// leaving), each write keeping the one before as <name>.save.bak.

// ErrGameExists is returned when a game of that name is already saved.
var ErrGameExists = errors.New("there's already a game called that")

// GameInfo is one game in the games list, read from its header.
type GameInfo struct {
	SaveInfo
	Scenario world.ScenarioInfo
	Date     time.Time // the game day; zero for a save from before games had one
	Stars    float32
	Outcome  world.Outcome
}

// GamePath is where the game called name is saved.
func GamePath(name string) string {
	return filepath.Join(SavesDir(), SanitizeSaveName(name)+SaveExt)
}

// GameExists reports whether a game called name is saved.
func GameExists(name string) bool {
	_, err := os.Stat(GamePath(name))
	return err == nil
}

// NewGameName is the name for a new game of a scenario called base: base
// itself, or "base 2", "base 3" when those are taken.
func NewGameName(base string) string {
	base = SanitizeSaveName(base)
	if base == "" {
		base = "Game"
	}
	name := base
	for n := 2; GameExists(name); n++ {
		name = fmt.Sprintf("%s %d", base, n)
	}
	return name
}

// SaveGame writes w as the game called name, keeping the previous save
// as a backup.
func SaveGame(name string, w *world.World, cam *CameraData) error {
	path := GamePath(name)
	data := worldToData(w, false)
	data.Game = &GameData{
		Date:    gameDate(w).Format(startDateLayout),
		Stars:   w.Rating,
		Outcome: uint8(w.Outcome),
	}
	if cam != nil {
		c := *cam
		data.Camera = &c
	}
	if _, err := os.Stat(path); err == nil {
		bak := path + ".bak"
		os.Remove(bak)
		if err := os.Link(path, bak); err != nil {
			copyFile(path, bak) // no hard links here: a copy will do
		}
	}
	return WriteScenarioData(path, data)
}

// gameDate is the game day w's clock is on (sim.DateAt, which save can't
// import).
func gameDate(w *world.World) time.Time {
	return world.GameDayAt(world.GameDayIndex(w.StartDate) + int(math.Floor(w.SimTime/world.SecondsPerSimDay)))
}

// ListGames returns every game in SavesDir, most recently played first.
func ListGames() []GameInfo {
	saves := ListSaves()
	out := make([]GameInfo, 0, len(saves))
	for _, s := range saves {
		g := GameInfo{SaveInfo: s}
		if data, err := readHeader(s.Path); err == nil {
			g.Scenario = scenarioInfoOf(data)
			if gd := data.Game; gd != nil {
				g.Date, _ = time.Parse(startDateLayout, gd.Date)
				g.Stars = gd.Stars
				g.Outcome = world.Outcome(gd.Outcome)
			}
		}
		out = append(out, g)
	}
	return out
}

// RenameGame renames the game called from to to, refusing a name another
// game has (ErrGameExists). Only the case may change on its own.
func RenameGame(from, to string) error {
	to = SanitizeSaveName(to)
	if to == "" {
		return errors.New("a game needs a name")
	}
	if to == from {
		return nil
	}
	if !strings.EqualFold(to, from) && GameExists(to) {
		return ErrGameExists
	}
	src, dst := GamePath(from), GamePath(to)
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	os.Rename(src+".bak", dst+".bak")
	return nil
}

// CopyGame saves a copy of the game called name as "name copy" ("name
// copy 2", ...) and returns the copy's name.
func CopyGame(name string) (string, error) {
	base := name + " copy"
	to := base
	for n := 2; GameExists(to); n++ {
		to = fmt.Sprintf("%s %d", base, n)
	}
	src, dst := GamePath(name), GamePath(to)
	if err := copyFile(src, dst); err != nil {
		return "", err
	}
	// Last played when the original was, so Continue doesn't pick the copy.
	if fi, err := os.Stat(src); err == nil {
		os.Chtimes(dst, fi.ModTime(), fi.ModTime())
	}
	return to, nil
}

// DeleteGame removes the game called name and its backup.
func DeleteGame(name string) error {
	path := GamePath(name)
	os.Remove(path + ".bak")
	return os.Remove(path)
}

// copyFile copies src to dst, atomically.
func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeAtomic(dst, b)
}
