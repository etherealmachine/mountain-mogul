package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Units controls the display unit system throughout the game.
type Units int

const (
	Imperial Units = iota
	Metric
)

// Settings holds all user-configurable preferences. The zero value is valid
// and matches the defaults applied by Init.
type Settings struct {
	Units Units `json:"units"`
}

var global = &Settings{Units: Imperial}

// Get returns the live settings. Callers should not cache the pointer.
func Get() *Settings { return global }

// Init loads settings from disk, applying defaults for any missing fields.
// Safe to call before the window is created.
func Init() {
	path := filePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, global)
}

// Save writes the current settings to disk.
func Save() error {
	path := filePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(global)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func filePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "mountain-mogul", "settings.json")
}

// FormatTemp formats a Celsius temperature as an integer in the active unit
// system, with no suffix (the column is narrow; the player knows their setting).
func FormatTemp(tempC float32) string {
	if global.Units == Imperial {
		return fmt.Sprintf("%d", int(tempC*9/5+32))
	}
	return fmt.Sprintf("%d", int(tempC))
}

// TempUnit returns "°F" or "°C" for the active unit system.
func TempUnit() string {
	if global.Units == Imperial {
		return "°F"
	}
	return "°C"
}

// FormatSpeed formats a speed in m/s as a labelled string in the active unit system.
func FormatSpeed(ms float32) string {
	if global.Units == Imperial {
		return fmt.Sprintf("%.1f mph", ms*2.23694)
	}
	return fmt.Sprintf("%.1f km/h", ms*3.6)
}

// FormatDepth formats a snow/water depth in metres as a labelled string.
func FormatDepth(m float32) string {
	if global.Units == Imperial {
		return fmt.Sprintf("%.2f ft", m*3.28084)
	}
	return fmt.Sprintf("%.2f m", m)
}

// FormatElevation formats a terrain elevation in metres as a labelled string.
func FormatElevation(m float32) string {
	if global.Units == Imperial {
		return fmt.Sprintf("%d ft", int(m*3.28084))
	}
	return fmt.Sprintf("%d m", int(m))
}
