package render

import (
	"fmt"
	"path/filepath"

	"github.com/go-gl/mathgl/mgl32"
)

// Icon names — single source of truth so callers don't pass arbitrary
// strings. The name maps 1:1 to the file under assets/icons/<name>.png.
type IconName string

const (
	IconCableCar       IconName = "cable-car"
	IconTreeEvergreen  IconName = "tree-evergreen"
	IconHouse          IconName = "house"
	IconAxe            IconName = "axe"
	IconTrash          IconName = "trash"
	IconArrowFatUp     IconName = "arrow-fat-up"
	IconArrowFatDown   IconName = "arrow-fat-down"
	IconGlobe          IconName = "globe"
	IconFloppyDisk     IconName = "floppy-disk"
	IconGear           IconName = "gear"
	IconPause          IconName = "pause"
	IconPlay           IconName = "play"
	IconFastForward    IconName = "fast-forward"
	IconCoin           IconName = "coin"
	IconUsers          IconName = "users"
	IconHeart          IconName = "heart"
	IconSun            IconName = "sun"
	IconCloudSun       IconName = "cloud-sun"
	IconCloud          IconName = "cloud"
	IconCloudSnow      IconName = "cloud-snow"
	IconSnowflake      IconName = "snowflake"
	IconCloudLightning IconName = "cloud-lightning"
	IconArrowRight     IconName = "arrow-right"
	IconCocktail       IconName = "chart-bar" // placeholder for bar tool

	// Overlay panel — the right-side stack of view toggles. `Stack` is the
	// panel's own toggle in the top bar; the rest are the individual overlay
	// modes addressable from the panel.
	IconStack     IconName = "stack"      // overlay panel toggle (top bar)
	IconChartLine IconName = "chart-line" // contour
	IconChartBar  IconName = "chart-bar"  // charts window toggle (top bar) + bar-chart tab
	IconTriangle  IconName = "triangle"   // slope debug
	IconWaves     IconName = "waves"      // snow depth
	IconBroom     IconName = "broom"      // grooming
	IconGridFour  IconName = "grid-four"  // packed snow
	IconDrop      IconName = "drop"       // ice
	IconDotsNine  IconName = "dots-nine"  // moguls

	// Buildings / equipment
	IconGarage IconName = "garage" // equipment shed (snowcats, snowmobiles)

	// Infrastructure
	IconRoad IconName = "road" // road placement tool
	IconFlag IconName = "flag" // road-network edge-connection marker (editor only)
)

// allIcons is the full set loaded at startup. Adding an icon requires
// extracting it into assets/icons/ and adding the constant + this list
// entry — a single-place change that keeps the registry exhaustive.
var allIcons = []IconName{
	IconCableCar, IconTreeEvergreen, IconHouse, IconAxe, IconTrash,
	IconArrowFatUp, IconArrowFatDown, IconGlobe, IconFloppyDisk,
	IconGear, IconPause, IconPlay, IconFastForward,
	IconCoin, IconUsers, IconHeart,
	IconSun, IconCloudSun, IconCloud, IconCloudSnow, IconSnowflake, IconCloudLightning,
	IconArrowRight, IconCocktail,
	IconStack, IconChartLine, IconChartBar, IconTriangle, IconWaves, IconBroom, IconGridFour, IconDrop, IconDotsNine,
	IconGarage,
	IconRoad,
	IconFlag,
}

// LoadIcons populates r.icons from assetDir/icons/. Missing files log a
// warning (LoadIconTexture falls back to a 1×1 white texture) so a typo
// produces a visible placeholder rather than a crash.
func (r *Renderer) LoadIcons() {
	r.icons = make(map[IconName]uint32, len(allIcons))
	dir := filepath.Join(r.assetDir, "icons")
	for _, name := range allIcons {
		path := filepath.Join(dir, string(name)+".png")
		texID, err := LoadIconTexture(path)
		if err != nil {
			fmt.Printf("LoadIcons: %s: %v\n", path, err)
		}
		r.icons[name] = texID
	}
}

// DrawIcon renders the named icon into the rect (x, y, size, size) tinted
// by color. The icon is drawn via the UI textured-rect path; alpha blending
// must already be enabled by the caller (DrawUI does this).
func (r *Renderer) DrawIcon(name IconName, x, y, size float32, color mgl32.Vec4) {
	texID, ok := r.icons[name]
	if !ok || texID == 0 {
		// Visible placeholder: fall through to a coloured square so a
		// missing icon is obvious rather than invisible.
		r.DrawColorRect(x, y, size, size, color)
		return
	}
	r.DrawTexturedRect(x, y, size, size, texID, color)
}
