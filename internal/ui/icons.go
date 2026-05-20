package ui

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/render"
)

// Icon helpers — thin wrappers around r.DrawIcon so callers in this
// package don't need to know the IconName constants. All icons live as
// PNGs under assets/icons/, loaded at renderer init by render.LoadIcons.

func IconPause(r *render.Renderer, x, y, size float32, color mgl32.Vec4) {
	r.DrawIcon(render.IconPause, x, y, size, color)
}

func IconPlay(r *render.Renderer, x, y, size float32, color mgl32.Vec4) {
	r.DrawIcon(render.IconPlay, x, y, size, color)
}

// IconFastForward draws a single fast-forward glyph; the speed-button
// label ("2x" / "4x") under the icon disambiguates the multiplier so the
// icon itself doesn't need to.
func IconFastForward(r *render.Renderer, x, y, size float32, color mgl32.Vec4) {
	r.DrawIcon(render.IconFastForward, x, y, size, color)
}

func IconGear(r *render.Renderer, x, y, size float32, color mgl32.Vec4) {
	r.DrawIcon(render.IconGear, x, y, size, color)
}

func IconCoin(r *render.Renderer, x, y, size float32, color mgl32.Vec4) {
	r.DrawIcon(render.IconCoin, x, y, size, color)
}

func IconPerson(r *render.Renderer, x, y, size float32, color mgl32.Vec4) {
	r.DrawIcon(render.IconUsers, x, y, size, color)
}

func IconHeart(r *render.Renderer, x, y, size float32, color mgl32.Vec4) {
	r.DrawIcon(render.IconHeart, x, y, size, color)
}

func IconArrow(r *render.Renderer, x, y, size float32, color mgl32.Vec4) {
	r.DrawIcon(render.IconArrowRight, x, y, size, color)
}

// WeatherKind is the UI-side enum for weather icon selection. The scenario
// translates from sim.WeatherState; the two packages can't import each other.
type WeatherKind int

const (
	WKSunny WeatherKind = iota
	WKCloudy
	WKSnow
	WKStorm
	WKRain
)

// DrawWeatherIcon picks a phosphor icon per weather kind. Tints lean toward
// the natural colour of each phenomenon (warm sun, neutral cloud, cool
// snow, dim storm) but the alpha mask makes the choice fairly forgiving.
func DrawWeatherIcon(r *render.Renderer, kind WeatherKind, x, y, size float32) {
	switch kind {
	case WKSunny:
		r.DrawIcon(render.IconSun, x, y, size, mgl32.Vec4{1.0, 0.85, 0.25, 1})
	case WKCloudy:
		r.DrawIcon(render.IconCloudSun, x, y, size, mgl32.Vec4{0.88, 0.90, 0.97, 1})
	case WKSnow:
		r.DrawIcon(render.IconCloudSnow, x, y, size, mgl32.Vec4{0.85, 0.92, 1.0, 1})
	case WKStorm:
		// Two cloud-snow icons side by side at reduced size to read as heavy snow.
		half := size * 0.62
		gap := size * 0.08
		r.DrawIcon(render.IconCloudSnow, x, y+size-half, half, mgl32.Vec4{0.70, 0.82, 1.0, 1})
		r.DrawIcon(render.IconCloudSnow, x+half+gap, y+size-half, half, mgl32.Vec4{0.70, 0.82, 1.0, 1})
	case WKRain:
		r.DrawIcon(render.IconDrop, x, y, size, mgl32.Vec4{0.55, 0.75, 0.95, 1})
	}
}
