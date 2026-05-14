package scene

import (
	"math"
	"math/rand"

	"mountain-mogul/internal/world"
)

// GenerateSnowCover overwrites Terrain.SnowAccumulation with a per-cell field
// shaped by physically-motivated factors:
//
//   - Snowline gate    — depth tapers off below a user-set elevation cutoff.
//   - Lapse rate       — depth keeps growing with elevation above the snowline
//                        (longer accumulation season, less melt, more snow vs rain).
//   - Slope shed       — steep slopes lose snow to sluffing / avalanche; cliffs bare.
//   - Curvature bias   — bowls and gullies catch drifts; ridges scour.
//   - Drainage drift   — MFD flow accumulation: gullies and lee bowls catch blown snow.
//   - Wind aspect      — leeward slopes accumulate; windward slopes scour.
//   - Treeline expo.   — below treeline, the canopy buffers wind so the
//                        drainage / wind / aspect terms are attenuated.
//   - Noise overlay    — low-frequency variation so the field doesn't read as a pure function.
//
// Other snow-state scalars (Grooming, Packed, Ice, MogulSize) are left alone:
// this is a depth-only pass. Pair with the manual brushes for finer work.
// Result is in metres; cliffs and very steep faces drop to zero.
//
// Parameters:
//   - maxDepth      — the cap on snow depth in metres for the most-favoured cells.
//                     All other modifiers reduce or boost this value as a fraction
//                     that's clamped to [0, 1] before scaling.
//   - snowlineFrac  — elevation cutoff, as a fraction of the map's elevation
//                     range. 0 = no gating (snow everywhere), 1 = no snow
//                     anywhere. The gate has a soft 15 %-of-range transition.
//   - treelineFrac  — elevation above which trees can't grow, as a fraction
//                     of the map's elevation range. Wind-driven terms
//                     (drainage drift, wind aspect) ramp up across the
//                     treeline; below, the canopy buffers them.
//   - windDeg       — wind direction, degrees clockwise from north (the
//                     direction the wind blows TOWARDS). 0 = north (-Z),
//                     90 = east (+X), 180 = south (+Z), 270 = west (-X).
//                     Slopes facing along the wind vector (lee side of ridges)
//                     accumulate; slopes facing against it scour.
//   - seed          — drives the low-frequency noise overlay.
func GenerateSnowCover(t *world.Terrain, maxDepth, snowlineFrac, treelineFrac, windDeg float32, seed int64) {
	computeElevFields(t).generateSnowCover(t, maxDepth, snowlineFrac, treelineFrac, windDeg, seed)
}

// generateSnowCover is the cached-fields variant: callers that need to
// re-run the generator several times in a row (e.g. live slider drag)
// can build an *elevFields once and call this directly to skip the
// O(N log N) flow-accumulation pass.
func (f *elevFields) generateSnowCover(t *world.Terrain, maxDepth, snowlineFrac, treelineFrac, windDeg float32, seed int64) {
	if maxDepth < 0 {
		maxDepth = 0
	}
	if snowlineFrac < 0 {
		snowlineFrac = 0
	} else if snowlineFrac > 1 {
		snowlineFrac = 1
	}
	if treelineFrac < 0 {
		treelineFrac = 0
	} else if treelineFrac > 1 {
		treelineFrac = 1
	}

	rng := rand.New(rand.NewSource(seed))
	hashSeed := int(rng.Int31())

	minE, maxE := f.minE, f.maxE
	span := maxE - minE
	curv := f.curv
	flow := f.flow

	// Wind unit vector in the XZ plane. World coordinates: +X east, +Z south.
	// windDeg increases clockwise from north, so the wind blows toward
	// (sin θ, -cos θ) in (X, Z).
	windRad := float64(windDeg) * math.Pi / 180.0
	windX := float32(math.Sin(windRad))
	windZ := float32(-math.Cos(windRad))

	// Low-frequency noise overlay — patchScale 40 ≈ 200 m features at 5 m cells.
	const patchScale = float32(40)

	// Snowline gate band: snowline edges fade in over this fraction of the
	// elevation span so the snow line isn't a perfectly horizontal hard cut.
	const snowlineBand = float32(0.15)

	// Treeline transition band — same idea, but for the wind-exposure ramp.
	const treelineBand = float32(0.20)

	// Lapse rate: how much extra snow piles on with elevation above the
	// snowline. lapseFloor is the depth multiplier right at the snowline
	// edge; cells at the very top of the elevation range reach 1.0. With
	// lapseFloor = 0.20 the peak-to-snowline ratio is 5×, roughly matching
	// real mid-latitude ski-mountain snowpack profiles.
	const lapseFloor = float32(0.20)

	// Below-treeline attenuation on the wind-driven terms. 1.0 means full
	// strength above treeline; floorAtt is the multiplier well below
	// treeline (canopy reduces wind redistribution but doesn't fully kill it).
	const flowExposureFloor = float32(0.40)
	const windExposureFloor = float32(0.30)

	// Slope shed thresholds — rise/run, dimensionless. 0.7 ≈ 35°, 1.4 ≈ 54°.
	const slopeStart = float32(0.7)
	const slopeEnd = float32(1.4)

	// Curvature bias — bowls deposit more than ridges strip.
	const driftBoost = float32(0.60)
	const ridgePenalty = float32(0.40)

	// Drainage drift — gullies and lee bowls collect blown snow. Smoothstep
	// gate on the log-normalised flow so only the strong drainage paths
	// receive the boost; the flat plains below don't all read as drift.
	const flowDriftBoost = float32(0.50)
	const flowLow = float32(0.40)
	const flowHigh = float32(0.90)

	// Wind aspect — leeward slopes deposit up to +scale, windward up to -scale.
	// Effect is gated by slope strength so flats are unaffected by wind.
	const windScale = float32(0.50)

	// Noise amplitude: ±20% multiplicative.
	const noiseAmp = float32(0.20)

	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			elev := t.Cells[x][z].GroundElevation
			gx, gz := gradientAt(t, x, z)
			slope := float32(math.Sqrt(float64(gx*gx + gz*gz)))

			d := float32(1.0)

			// Elevation fraction once — drives both the snowline gate and
			// the lapse + treeline-exposure terms below.
			elevFrac := float32(0)
			if span > 0 {
				elevFrac = (elev - minE) / span
				if elevFrac < 0 {
					elevFrac = 0
				} else if elevFrac > 1 {
					elevFrac = 1
				}
			}

			// Snowline gate. snowlineFrac near zero is treated as "no gate"
			// so the slider's bottom end gives blanket coverage.
			if snowlineFrac > 0.01 {
				d *= smoothstep32(snowlineFrac, snowlineFrac+snowlineBand, elevFrac)
			}

			// Lapse term: above the snowline, depth keeps climbing with
			// elevation. Just-above-snowline cells get lapseFloor of the
			// max; peaks get the full max. Below the snowline this is
			// clamped to lapseFloor but the gate above has already zeroed
			// d so the lapse multiplier doesn't matter there.
			aboveRange := 1 - snowlineFrac
			if aboveRange < 0.05 {
				aboveRange = 0.05
			}
			above := (elevFrac - snowlineFrac) / aboveRange
			if above < 0 {
				above = 0
			} else if above > 1 {
				above = 1
			}
			d *= lapseFloor + (1-lapseFloor)*above

			// Slope shed.
			if slope >= slopeEnd {
				d = 0
			} else if slope > slopeStart {
				d *= 1 - (slope-slopeStart)/(slopeEnd-slopeStart)
			}

			if d > 0 {
				// Treeline exposure: 0 well below treeline, 1 above.
				// Modulates the wind-driven terms (drainage drift, wind
				// aspect) so canopy-covered ground holds a more uniform
				// snowpack while alpine ground varies wildly with terrain.
				exposure := smoothstep32(treelineFrac-treelineBand/2, treelineFrac+treelineBand/2, elevFrac)
				flowExposure := flowExposureFloor + (1-flowExposureFloor)*exposure
				windExposure := windExposureFloor + (1-windExposureFloor)*exposure

				// Curvature bias.
				c := curv[x*t.Height+z]
				if c > 0 {
					d *= 1 + smoothstep32(0.15, 0.85, c)*driftBoost
				} else if c < 0 {
					d *= 1 - smoothstep32(0.15, 0.85, -c)*ridgePenalty
				}

				// Drainage drift, attenuated below treeline.
				fl := flow[x*t.Height+z]
				d *= 1 + smoothstep32(flowLow, flowHigh, fl)*flowDriftBoost*flowExposure

				// Wind aspect, attenuated below treeline. Downhill direction
				// = -gradient / |gradient|; dot with wind tells us how
				// leeward (or windward) the face is. Effect ramps with slope
				// so flats stay neutral.
				if slope > 0.05 {
					downhillX := -gx / slope
					downhillZ := -gz / slope
					alignment := downhillX*windX + downhillZ*windZ
					slopeStrength := smoothstep32(0.05, 0.30, slope)
					d *= 1 + windScale*alignment*slopeStrength*windExposure
				}

				// Noise overlay — multiplicative, centred on 1.
				n := fbm2D(float32(x)/patchScale, float32(z)/patchScale, 3, hashSeed)
				d *= 1 + (n-0.5)*2*noiseAmp
			}

			if d < 0 {
				d = 0
			} else if d > 1 {
				d = 1
			}
			// maxDepth is in visible-metres at fresh-powder density; convert
			// to SWE so the accumulation field carries the conserved quantity.
			acc := maxDepth * d * world.SnowDensity(t.Cells[x][z].Packed)
			t.Cells[x][z].SnowAccumulation = acc
		}
	}
	t.SnowDirty = true
}

// gradientAt returns the elevation gradient (∂e/∂x, ∂e/∂z) at cell (x, z)
// using central differences with edge clamping. Same edge handling as
// slopeAt; the magnitude of this vector equals slopeAt's result.
func gradientAt(t *world.Terrain, x, z int) (float32, float32) {
	const cellSize = float32(5.0)
	x0, x1 := x-1, x+1
	if x0 < 0 {
		x0 = 0
	}
	if x1 >= t.Width {
		x1 = t.Width - 1
	}
	z0, z1 := z-1, z+1
	if z0 < 0 {
		z0 = 0
	}
	if z1 >= t.Height {
		z1 = t.Height - 1
	}
	dxRun := float32(x1-x0) * cellSize
	dzRun := float32(z1-z0) * cellSize
	gx := float32(0)
	if dxRun > 0 {
		gx = (t.Cells[x1][z].GroundElevation - t.Cells[x0][z].GroundElevation) / dxRun
	}
	gz := float32(0)
	if dzRun > 0 {
		gz = (t.Cells[x][z1].GroundElevation - t.Cells[x][z0].GroundElevation) / dzRun
	}
	return gx, gz
}
