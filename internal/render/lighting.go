package render

import (
	"math"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

// Lighting is the frame's key light, fill light and sky colour, fed to the
// terrain, static and dynamic shaders (lighting.glsl) and the clear colour.
type Lighting struct {
	SunDir   mgl32.Vec3 // unit vector toward the key light (sun, or moon at night)
	SunColor mgl32.Vec3 // key light colour × intensity
	Ambient  mgl32.Vec3 // sky fill from above, colour × intensity
	Ground   mgl32.Vec3 // bounce fill from below (sunlit snow), colour × intensity
	Sky      mgl32.Vec3 // background clear colour
	Night    float32    // 0 in daylight → 1 after dusk; scales vehicle lamps
	Shadows  bool       // terrain casts shadows from SunDir (horizon map)
}

// DefaultLighting is the fixed midday light used when there's no clock
// (editor, menus).
var DefaultLighting = Lighting{
	SunDir:   mgl32.Vec3{0.6, 1.0, 0.4}.Normalize(),
	SunColor: mgl32.Vec3{1, 1, 1},
	Ambient:  mgl32.Vec3{0.42, 0.45, 0.50},
	Ground:   mgl32.Vec3{0.42, 0.41, 0.40},
	Sky:      weatherSky(0),
}

// Light-level constants. Night is kept readable rather than realistic:
// a moonlit resort you can still manage.
var (
	noonSun      = mgl32.Vec3{1.00, 0.98, 0.95}
	horizonSun   = mgl32.Vec3{1.00, 0.55, 0.30}
	moonLight    = mgl32.Vec3{0.16, 0.19, 0.30}
	dayAmbient   = mgl32.Vec3{0.42, 0.45, 0.50}
	duskAmbient  = mgl32.Vec3{0.32, 0.29, 0.36}
	cloudAmbient = mgl32.Vec3{0.35, 0.36, 0.38}
	bounceTint   = mgl32.Vec3{1.00, 0.97, 0.92}
	nightAmbient = mgl32.Vec3{0.05, 0.06, 0.11}
	nightSky     = mgl32.Vec3{0.02, 0.03, 0.08}
	glowSky      = mgl32.Vec3{0.95, 0.62, 0.45}
)

// referenceFlatLight is how brightly daylight lights flat ground after
// exposure; toneMap in lighting.glsl rolls off what goes past white.
// Daylight exposure scales low winter sun back toward it, as eyes and
// cameras do, up to maxExposure.
const (
	referenceFlatLight = 0.85
	maxExposure        = 1.8
)

// weatherSky is the daytime sky colour for a weather overlay state
// (mirrors sim.WeatherState).
func weatherSky(weather int) mgl32.Vec3 {
	switch weather {
	case 1: // overcast
		return mgl32.Vec3{0.62, 0.63, 0.68}
	case 2: // light snow
		return mgl32.Vec3{0.72, 0.74, 0.80}
	case 3: // heavy snow
		return mgl32.Vec3{0.58, 0.60, 0.64}
	case 4: // rain
		return mgl32.Vec3{0.45, 0.50, 0.58}
	default: // clear
		return mgl32.Vec3{0.635, 0.682, 0.918}
	}
}

// CanopySnowFor is how much snow loads the branches under a weather
// overlay state (mirrors sim.WeatherState): full in heavy snow, partial
// in light snow, none otherwise.
func CanopySnowFor(weather int) float32 {
	switch weather {
	case 2:
		return 0.6
	case 3:
		return 1
	default:
		return 0
	}
}

// weatherDirect is how much of the direct sun gets through each weather
// state; the rest turns into diffuse fill.
func weatherDirect(weather int) float32 {
	switch weather {
	case 1:
		return 0.40
	case 2:
		return 0.30
	case 3:
		return 0.18
	case 4:
		return 0.30
	default:
		return 1
	}
}

// SunLighting builds the lighting for a sun at sunDir (unit vector toward
// the sun, world space, Y up) under the given weather state. Direct sun
// fades out and reddens as the sun sets; through civil twilight (sun 0–6°
// below the horizon) the sky glows and dims, then a dim blue moonlight
// from the anti-sun direction takes over.
func SunLighting(sunDir mgl32.Vec3, weather int) Lighting {
	elev := float32(math.Asin(float64(mgl32.Clamp(sunDir[1], -1, 1))))
	const deg = math.Pi / 180
	dayF := smoothstep(-10*deg, 2*deg, elev)      // sky / ambient: twilight to day
	sunF := smoothstep(-0.5*deg, 7*deg, elev)     // direct sun
	moonF := 1 - smoothstep(-8*deg, -2*deg, elev) // moon takes over as the sun goes down
	warm := smoothstep(1*deg, 12*deg, elev)       // sun colour: horizon orange → white
	direct := weatherDirect(weather)
	clear := float32(0)
	if weather == 0 {
		clear = 1
	}

	var l Lighting
	if elev > -3*deg {
		l.SunDir = sunDir
		if l.SunDir[1] < 0.05 {
			l.SunDir[1] = 0.05 // graze rather than light from below
			l.SunDir = l.SunDir.Normalize()
		}
		l.SunColor = lerp3(horizonSun, noonSun, warm).Mul(sunF * direct)
	} else {
		moon := sunDir.Mul(-1)
		if moon[1] < 0.35 {
			moon[1] = 0.35
		}
		l.SunDir = moon.Normalize()
		l.SunColor = moonLight.Mul(moonF * (0.4 + 0.6*direct))
	}

	// Cloud turns lost direct light into fill.
	fill := 1 + (1-direct)*0.8*sunF
	twilight := 1 - smoothstep(0, 12*deg, elev)       // strongest near the horizon
	dayAmb := lerp3(cloudAmbient, dayAmbient, direct) // blue sky fill turns grey under cloud
	dayAmb = lerp3(dayAmb, duskAmbient, twilight*dayF)
	l.Ambient = lerp3(nightAmbient, dayAmb.Mul(fill), dayF)
	sunUp := float32(math.Max(float64(l.SunDir[1]), 0))
	l.Ground = l.Ambient.Mul(0.55).Add(mul3(l.SunColor, bounceTint).Mul(0.30 * sunUp))

	// Daylight exposure: lift a low sun's flat-ground light toward the
	// reference, fading out through twilight so dusk still darkens.
	if elev > -5*deg {
		flat := (l.Ambient[0]+l.Ambient[1]+l.Ambient[2])/3 +
			0.85*l.SunColor[1]*l.SunDir[1]
		target := referenceFlatLight * (0.85 + 0.15*direct) // overcast stays a little grey
		k := mgl32.Clamp(target/flat, 1, maxExposure)
		k = 1 + (k-1)*smoothstep(-5*deg, 3*deg, elev)
		l.SunColor = l.SunColor.Mul(k)
		l.Ambient = l.Ambient.Mul(k)
		l.Ground = l.Ground.Mul(k)
	}

	glow := twilight * smoothstep(-6*deg, 0, elev) * (0.35 + 0.65*clear)
	sky := lerp3(nightSky, weatherSky(weather), dayF)
	l.Sky = lerp3(sky, glowSky, glow*0.35)
	l.Night = 1 - smoothstep(-4*deg, 8*deg, elev)
	l.Shadows = true
	return l
}

// SetAntiAliasing turns multisampled rasterisation on or off. The window
// always has a multisampled framebuffer; off skips the per-sample work.
func SetAntiAliasing(on bool) {
	if on {
		gl.Enable(gl.MULTISAMPLE)
	} else {
		gl.Disable(gl.MULTISAMPLE)
	}
}

func smoothstep(e0, e1, x float32) float32 {
	t := mgl32.Clamp((x-e0)/(e1-e0), 0, 1)
	return t * t * (3 - 2*t)
}

func lerp3(a, b mgl32.Vec3, t float32) mgl32.Vec3 {
	return a.Add(b.Sub(a).Mul(t))
}

func mul3(a, b mgl32.Vec3) mgl32.Vec3 {
	return mgl32.Vec3{a[0] * b[0], a[1] * b[1], a[2] * b[2]}
}

// light is the overall scene light level (1 ≈ clear midday), matching
// sceneLight() in lighting.glsl.
func (l Lighting) light() mgl32.Vec3 {
	return l.Ambient.Add(l.SunColor.Mul(0.85)).Mul(1 / 1.1)
}

func (l Lighting) apply(s *Shader) {
	s.SetVec3("uSunDir", l.SunDir)
	s.SetVec3("uSunColor", l.SunColor)
	s.SetVec3("uAmbient", l.Ambient)
	s.SetVec3("uGroundFill", l.Ground)
}
