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
	// HideDailyReport stops the profit/loss recap opening each midnight.
	HideDailyReport bool `json:"hide_daily_report,omitempty"`
	// NoAntiAliasing turns off multisampled edges, for slower GPUs.
	NoAntiAliasing bool `json:"no_anti_aliasing,omitempty"`
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

// FormatClock formats an hour of day (0..24) as "9:42 AM" under Imperial
// units and "09:42" under Metric.
func FormatClock(hour float64) string {
	mins := int(hour*60) % (24 * 60)
	if mins < 0 {
		mins += 24 * 60
	}
	h, m := mins/60, mins%60
	if global.Units == Metric {
		return fmt.Sprintf("%02d:%02d", h, m)
	}
	suffix := "AM"
	if h >= 12 {
		suffix = "PM"
	}
	h12 := h % 12
	if h12 == 0 {
		h12 = 12
	}
	return fmt.Sprintf("%d:%02d %s", h12, m, suffix)
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

// FormatArea formats a count of grid cells as a labelled area string.
// Each cell is 5 m × 5 m = 25 sq m.
func FormatArea(cells int) string {
	sqm := float64(cells) * 25.0
	if global.Units == Imperial {
		return fmt.Sprintf("%.1f acres", sqm/4046.856)
	}
	return fmt.Sprintf("%.2f sq km", sqm/1_000_000)
}
