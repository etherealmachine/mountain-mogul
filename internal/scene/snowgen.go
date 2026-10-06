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
//     (longer accumulation season, less melt, more snow vs rain).
//   - Slope shed       — steep slopes lose snow to sluffing / avalanche; cliffs bare.
//   - Curvature bias   — bowls and gullies catch drifts; ridges scour.
//   - Drainage drift   — MFD flow accumulation: gullies and lee bowls catch blown snow.
//   - Wind aspect      — leeward slopes accumulate; windward slopes scour.
//   - Treeline expo.   — below treeline, the canopy buffers wind so the
//     drainage / wind / aspect terms are attenuated.
//   - Noise overlay    — low-frequency variation so the field doesn't read as a pure function.
//
// Other snow-state scalars (Grooming, Packed, Ice, MogulSize) are left alone:
// this is a depth-only pass. Pair with the manual brushes for finer work.
// Result is in metres; cliffs and very steep faces drop to zero.
//
// Parameters:
//   - maxDepth      — the cap on snow depth in metres for the most-favoured cells.
//     All other modifiers reduce or boost this value as a fraction
//     that's clamped to [0, 1] before scaling.
//   - snowlineFrac  — elevation cutoff, as a fraction of the map's elevation
//     range. 0 = no gating (snow everywhere), 1 = no snow
//     anywhere. The gate has a soft 15 %-of-range transition.
//   - treelineFrac  — elevation above which trees can't grow, as a fraction
//     of the map's elevation range. Wind-driven terms
//     (drainage drift, wind aspect) ramp up across the
//     treeline; below, the canopy buffers them.
//   - windDeg       — wind direction, degrees clockwise from north (the
//     direction the wind blows TOWARDS). 0 = north (-Z),
//     90 = east (+X), 180 = south (+Z), 270 = west (-X).
//     Slopes facing along the wind vector (lee side of ridges)
//     accumulate; slopes facing against it scour.
//   - seed          — drives the low-frequency noise overlay.
func GenerateSnowCover(t *world.Terrain, maxDepth, snowlineFrac, treelineFrac, windDeg float32, seed int64) {
	computeElevFields(t).generateSnowCover(t, maxDepth, snowlineFrac, treelineFrac, windDeg, seed)
}

// AddSnowLayer is like GenerateSnowCover but pushes a new layer onto the
// existing stack rather than replacing it. Used by the editor's "Add Storm"
// button to build up a multi-layer snowpack.
func AddSnowLayer(t *world.Terrain, kind world.SnowKind, maxDepth, snowlineFrac, treelineFrac, windDeg float32, seed int64) {
	computeElevFields(t).addSnowLayer(t, kind, maxDepth, snowlineFrac, treelineFrac, windDeg, seed)
}

// addSnowLayerCached is the cached-fields variant for the editor's hot path
// (avoids recomputing flow accumulation on every "Add Storm" click).
func addSnowLayerCached(f *elevFields, t *world.Terrain, kind world.SnowKind, maxDepth, snowlineFrac, treelineFrac, windDeg float32, seed int64) {
	f.addSnowLayer(t, kind, maxDepth, snowlineFrac, treelineFrac, windDeg, seed)
}

// generateSnowCover is the cached-fields variant.
func (f *elevFields) generateSnowCover(t *world.Terrain, maxDepth, snowlineFrac, treelineFrac, windDeg float32, seed int64) {
	f.applySnowAccum(t, maxDepth, snowlineFrac, treelineFrac, windDeg, seed, func(x, z int, acc float32) {
		t.Cells[x][z].Base = 0
		t.Cells[x][z].Top = world.SnowLayer{Accumulation: acc, Kind: world.KindBase}
	})
}

// addSnowLayer is the cached-fields push variant.
func (f *elevFields) addSnowLayer(t *world.Terrain, kind world.SnowKind, maxDepth, snowlineFrac, treelineFrac, windDeg float32, seed int64) {
	f.applySnowAccum(t, maxDepth, snowlineFrac, treelineFrac, windDeg, seed, func(x, z int, acc float32) {
		c := &t.Cells[x][z]
		if c.Top.Accumulation > 0 && c.Top.Kind == kind {
			c.Top.Accumulation += acc
		} else {
			c.Base += c.Top.Accumulation
			c.Top = world.SnowLayer{Accumulation: acc, Kind: kind}
		}
	})
}

// applySnowAccum runs the terrain-aware snow distribution algorithm and
// calls apply(x, z, acc) for each cell with the computed SWE accumulation.
func (f *elevFields) applySnowAccum(t *world.Terrain, maxDepth, snowlineFrac, treelineFrac, windDeg float32, seed int64, apply func(x, z int, acc float32)) {
	if maxDepth < 0 {
		maxDepth = 0
	}
	if snowlineFrac < 0 {
		snowlineFrac = 0
	} else if snowlineFrac > 1 {
		snowlineFrac = 1
	}
	drift := f.newSnowDrift(t, treelineFrac, windDeg, seed)

	// Snowline gate band: snowline edges fade in over this fraction of the
	// elevation span so the snow line isn't a perfectly horizontal hard cut.
	const snowlineBand = float32(0.15)

	// Lapse rate: how much extra snow piles on with elevation above the
	// snowline. lapseFloor is the depth multiplier right at the snowline
	// edge; cells at the very top of the elevation range reach 1.0. With
	// lapseFloor = 0.20 the peak-to-snowline ratio is 5×, roughly matching
	// real mid-latitude ski-mountain snowpack profiles.
	const lapseFloor = float32(0.20)

	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			d := float32(1.0)

			// Elevation fraction once — drives both the snowline gate and
			// the lapse term below.
			elevFrac := f.elevFrac(t.Cells[x][z].GroundElevation)

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

			if d > 0 {
				d *= drift.at(x, z, elevFrac)
			}

			if d < 0 {
				d = 0
			} else if d > 1 {
				d = 1
			}
			// Convert visible-metres at Powder density to SWE.
			acc := maxDepth * d * world.KindDensity(world.KindPowder)
			apply(x, z, acc)
		}
	}
	t.SnowDirty = true
}

// elevFrac is elev as a fraction of the map's elevation range, in [0, 1].
func (f *elevFields) elevFrac(elev float32) float32 {
	if span := f.maxE - f.minE; span > 0 {
		return min(max((elev-f.minE)/span, 0), 1)
	}
	return 0
}

// snowDrift is how the ground shapes a snowpack, as a multiplier on the
// depth the weather alone would leave: slopes shed it, bowls, gullies
// and lee faces collect what the wind blows off ridges and windward
// faces, with less of that below the treeline.
type snowDrift struct {
	f            *elevFields
	t            *world.Terrain
	treelineFrac float32
	windX, windZ float32
	hashSeed     int
}

// newSnowDrift is the drift for treelineFrac (see GenerateSnowCover),
// storms blowing towards windDeg, and noise from seed.
func (f *elevFields) newSnowDrift(t *world.Terrain, treelineFrac, windDeg float32, seed int64) snowDrift {
	rng := rand.New(rand.NewSource(seed))
	// Wind unit vector in the XZ plane. World coordinates: +X east, +Z south.
	// windDeg increases clockwise from north, so the wind blows toward
	// (sin θ, -cos θ) in (X, Z).
	windRad := float64(windDeg) * math.Pi / 180.0
	return snowDrift{
		f: f, t: t,
		treelineFrac: min(max(treelineFrac, 0), 1),
		windX:        float32(math.Sin(windRad)),
		windZ:        float32(-math.Cos(windRad)),
		hashSeed:     int(rng.Int31()),
	}
}

// at is the drift multiplier for cell (x, z) at elevFrac of the map's
// range: 0 on cliffs, around 1 on open ground, up to about 2.5 in lee
// gullies.
func (s snowDrift) at(x, z int, elevFrac float32) float32 {
	// Low-frequency noise overlay — patchScale 40 ≈ 200 m features at 5 m cells.
	const patchScale = float32(40)

	// Treeline transition band for the wind-exposure ramp.
	const treelineBand = float32(0.20)

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

	gx, gz := s.t.GradientAt(x, z)
	slope := float32(math.Sqrt(float64(gx*gx + gz*gz)))
	d := float32(1)

	// Slope shed.
	if slope >= slopeEnd {
		return 0
	} else if slope > slopeStart {
		d *= 1 - (slope-slopeStart)/(slopeEnd-slopeStart)
	}

	// Treeline exposure: 0 well below treeline, 1 above. Modulates the
	// wind-driven terms (drainage drift, wind aspect) so canopy-covered
	// ground holds a more uniform snowpack while alpine ground varies
	// wildly with terrain.
	exposure := smoothstep32(s.treelineFrac-treelineBand/2, s.treelineFrac+treelineBand/2, elevFrac)
	flowExposure := flowExposureFloor + (1-flowExposureFloor)*exposure
	windExposure := windExposureFloor + (1-windExposureFloor)*exposure

	// Curvature bias.
	i := x*s.t.Height + z
	if c := s.f.curv[i]; c > 0 {
		d *= 1 + smoothstep32(0.15, 0.85, c)*driftBoost
	} else if c < 0 {
		d *= 1 - smoothstep32(0.15, 0.85, -c)*ridgePenalty
	}

	// Drainage drift, attenuated below treeline.
	d *= 1 + smoothstep32(flowLow, flowHigh, s.f.flow[i])*flowDriftBoost*flowExposure

	// Wind aspect, attenuated below treeline. Downhill direction
	// = -gradient / |gradient|; dot with wind tells us how leeward (or
	// windward) the face is. Effect ramps with slope so flats stay neutral.
	if slope > 0.05 {
		alignment := (-gx*s.windX - gz*s.windZ) / slope
		d *= 1 + windScale*alignment*smoothstep32(0.05, 0.30, slope)*windExposure
	}

	// Noise overlay — multiplicative, centred on 1.
	n := fbm2D(float32(x)/patchScale, float32(z)/patchScale, 3, s.hashSeed)
	return max(d*(1+(n-0.5)*2*noiseAmp), 0)
}
