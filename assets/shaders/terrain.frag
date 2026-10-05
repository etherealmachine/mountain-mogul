#version 410 core

flat in vec3  vNormal;
in vec3  vWorldPos;
in float vSmoothY;
in float vAO;
in vec4  vSnow;             // (Grooming, Packed, Ice, MogulSize)
in float vSnowDepth;        // SnowDepth in metres
in vec3  vSmoothNormal;     // per-corner smoothed normal, interpolated across triangles
in float vInstabilityScore; // Cell.InstabilityScore(); 0=stable, 1.0=release threshold; -1 on map-edge skirts

uniform vec2  uBrushCenter;
uniform float uBrushRadius;
uniform vec2  uRangeCenter;
uniform float uRangeRadius;
uniform int   uOverlayMode;     // 0 = off, 1 = contour, 2 = slope debug
uniform vec3  uCameraPos;
uniform float uTime;
uniform float uTerrainMinY;
uniform float uTerrainMaxY;

// Sub-cell surface-detail texture, mirrored from world.SurfaceDetail.
// Channels:
//   R = skier track intensity (decays in sim time)
//   G = tree-well depth      (persistent until tree edits)
//   B, A = reserved
// uWorldSize is the terrain extent in metres = cells × 5, so
// vWorldPos.xz / uWorldSize is the texture's UV.
uniform sampler2D uSnowSurface;
uniform vec2      uWorldSize;

// Where snowcats groomed, mirrored from world.GroomMap at 1 m per texel:
//   R = groomed, GB = cat heading as (cos 2θ, sin 2θ) mapped to 0..1,
//   A = sideways offset from the pass centreline, ±2.5 m mapped to 0..1.
uniform sampler2D uGroomMap;

// Per-cell RGBA8 overlay: trails (green/blue/black) and grooming routes (cyan).
// One texel per terrain cell; linear filtering feathers cell edges.
// Alpha 0 = no overlay. UV = vWorldPos.xz / uWorldSize.
uniform sampler2D uCellOverlay;

out vec4 fragColor;

// Cell-hash for the sparkle and mogul passes — cheap integer mixing in float space.
float hash3(vec3 p) {
    p = fract(p * vec3(443.897, 441.423, 437.195));
    p += dot(p, p.yzx + 19.19);
    return fract((p.x + p.y) * p.z);
}

// 2D value-noise built from hash3 — used to drive mogul roughness.
// Named valueNoise (not noise2) so it doesn't collide with the GLSL
// built-in `vec2 noise2(genType)`; on macOS the strict compiler rejects
// the redeclaration with a different return type.
float valueNoise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    float a = hash3(vec3(i.x,     i.y,     0.0));
    float b = hash3(vec3(i.x + 1, i.y,     0.0));
    float c = hash3(vec3(i.x,     i.y + 1, 0.0));
    float d = hash3(vec3(i.x + 1, i.y + 1, 0.0));
    vec2 u = f * f * (3.0 - 2.0 * f);
    return mix(mix(a, b, u.x), mix(c, d, u.x), u.y);
}

// 3-octave FBM around valueNoise. Frequencies ×1, ×2, ×4; amplitudes
// ×1, ×0.5, ×0.25 (normalised to sum = 1.75 → output stays in [0, 1]).
// Pluggable replacement for the single-octave `valueNoise` calls in the
// powder/mogul kicks — the extra octaves break up the flat patches that
// a single-frequency noise leaves on close camera.
float fbmNoise(vec2 p) {
    float n = valueNoise(p);
    n += 0.5  * valueNoise(p * 2.0);
    n += 0.25 * valueNoise(p * 4.0);
    return n / 1.75;
}

// Value noise with its analytic gradient: (value, d/dp.x, d/dp.y) from one
// set of four hashes, so normal kicks don't need three offset samples.
vec3 valueNoiseD(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    float a = hash3(vec3(i.x,     i.y,     0.0));
    float b = hash3(vec3(i.x + 1, i.y,     0.0));
    float c = hash3(vec3(i.x,     i.y + 1, 0.0));
    float d = hash3(vec3(i.x + 1, i.y + 1, 0.0));
    vec2 u  = f * f * (3.0 - 2.0 * f);
    vec2 du = 6.0 * f * (1.0 - f);
    float k = a - b - c + d;
    return vec3(a + (b - a) * u.x + (c - a) * u.y + k * u.x * u.y,
                du.x * ((b - a) + k * u.y),
                du.y * ((c - a) + k * u.x));
}

// Gradient of fbmNoise. Octaves too fine to see at this pixel footprint
// (px, in noise units per pixel) are skipped.
vec2 fbmGrad(vec2 p, float px) {
    vec2 g = valueNoiseD(p).yz;
    if (px < 1.0) g += valueNoiseD(p * 2.0).yz;
    if (px < 0.5) g += valueNoiseD(p * 4.0).yz;
    return g / 1.75;
}

void main() {
    float grooming = clamp(vSnow.x, 0.0, 1.0);
    float packed   = clamp(vSnow.y, 0.0, 1.0);
    float ice      = clamp(vSnow.z, 0.0, 1.0);
    float mogul    = clamp(vSnow.w, 0.0, 1.0);
    // Metres of surface per pixel; fine detail fades out once it's sub-pixel.
    float pxM = max(length(dFdx(vWorldPos)), length(dFdy(vWorldPos)));
    // Avalanche-debris marker: ice > packed is a combination no weather-formed kind
    // produces (debris is written with packed=0.20, ice=0.45). Drives colour and
    // surface-roughness overrides below; suppresses powder and sparkle.
    float isDebris = step(packed + 0.02, ice);

    // Surface-detail sample — sub-cell features rendered from the
    // texture written by the simulation. R = skier tracks, G = tree-well
    // depth.
    vec4 surf = texture(uSnowSurface, vWorldPos.xz / uWorldSize);
    // Normalise raw R (additive splats, 0.25 per pass) so one pass reads
    // as half-intensity and two saturate. This caps the crossing peak at
    // the same visual level as a single pass, fixing the blobby gradient
    // artifact where ∇R → 0 at a local maximum. The cat wipes tracks as
    // it grooms, so any on corduroy are fresh wear and show in full.
    float track = smoothstep(0.0, 0.5, surf.r);
    float well  = surf.g;

    // Groomed look follows the cat's swath, faded by per-cell wear.
    vec4  gm       = texture(uGroomMap, vWorldPos.xz / uWorldSize);
    float stamp    = gm.r;
    float groomVis = stamp * smoothstep(0.1, 0.6, grooming);
    // A groomed cell's packing spills past the curved swath when it's
    // averaged to corners; outside the swath, read it as untouched snow.
    packed = mix(packed, min(packed, 0.2), grooming * (1.0 - stamp));

    // Tree wells: each well drops the visible snow column by up to
    // 0.5 m at the trunk so the surrounding ground starts showing
    // through at the base — matches what real spruce/fir wells look
    // like, where the canopy intercepts snow and the trunk radiates
    // just enough warmth to keep a moat clear.
    float effDepth = max(vSnowDepth - 0.5 * well, 0.0);

    // Smoothed per-corner normal across the whole surface. Snow reads as
    // continuous; cliffs (handled by the existing slope-based rocky tint
    // on the snow palette below) still look distinct via colour. The
    // vNormal (per-triangle flat) attribute is no longer read by the FS —
    // the mesh geometry already carries the lane-edge step via the
    // corner-Y averaging, and the smoothed normal lets the natural slope
    // do the lighting rather than chopping it into per-triangle facets.
    vec3 N = normalize(vSmoothNormal);
    float slope = clamp(N.y, 0.0, 1.0);                  // 1 = flat, 0 = vertical
    float h     = clamp((vWorldPos.y - uTerrainMinY) /
                        max(uTerrainMaxY - uTerrainMinY, 1.0), 0.0, 1.0);

    // Ground palette — what the bare rock/dirt/grass under the snow looks
    // like. Height-driven on flats; cliffs blend toward plain rock.
    vec3 rock       = vec3(0.34, 0.33, 0.32);
    vec3 groundLow  = vec3(0.45, 0.40, 0.28);            // dry meadow / tundra
    vec3 groundMid  = vec3(0.42, 0.38, 0.32);            // dirt / tundra
    vec3 groundHigh = vec3(0.52, 0.50, 0.46);            // exposed scree
    vec3 groundFlat = mix(groundLow, groundMid, smoothstep(0.20, 0.55, h));
         groundFlat = mix(groundFlat, groundHigh, smoothstep(0.65, 0.95, h));
    float rocky     = 1.0 - smoothstep(0.55, 0.78, slope);
    vec3  ground    = mix(groundFlat, rock, rocky);

    // Snow albedo. Packed reads slightly bluer / darker than fresh powder;
    // modulate before mixing with ground so packed never leaks onto bare
    // rock. Shade colour comes from the sky fill, not from elevation.
    vec3 snow      = vec3(0.95, 0.96, 0.98);
    // Broad patchiness so a snowfield isn't one flat sheet: sheltered
    // patches a touch brighter and warmer, scoured ones greyer and cooler.
    float patchN = valueNoise(vWorldPos.xz / 40.0 + vec2(71.3, 12.9)) * 0.65
                 + valueNoise(vWorldPos.xz / 9.0 + vec2(5.1, 33.7)) * 0.35;
    snow *= mix(vec3(0.95, 0.96, 0.99), vec3(1.02, 1.01, 0.99), patchN);
    // Packed-snow tint — noticeably cooler and a touch darker than
    // fresh powder. The geometric depth step at a groomed/powder
    // boundary is only a soft 2-cell ramp (corner Y is 4-cell averaged),
    // so the color shift is doing most of the work to mark a groomed
    // lane as visually distinct from its powder shoulder.
    //
    // World defaults to Packed=0.2 (powder reads untinted). A groomed
    // cell hits Packed=1.0 which lands ~22% darker / ~10% cooler — a
    // clear gray-blue cast vs the surrounding powder white. The
    // smoothstep starts at 0.30 so passing skier traffic (which slowly
    // raises Packed toward 1) starts to pick up the tint before full
    // grooming.
    vec3 packedTint = vec3(0.78, 0.82, 0.92);
    // Skier-track lanes read as compressed — bias `packed` toward 1 so
    // the track band picks up the same cool tint as a groomed cell.
    float effPacked = clamp(packed + track * 0.6, 0.0, 1.0);
    float packedMix = smoothstep(0.30, 1.0, effPacked) * 0.75;
    snow            = mix(snow, snow * packedTint, packedMix);
    // Plus a small absolute darken on the lane itself — real ski tracks
    // are perceptibly darker than the surrounding untracked snow.
    snow = snow * (1.0 - track * 0.03);

    // Powder character — two phases that together distinguish fresh
    // untracked snow from the cool, glassy-flat groomed lanes without
    // subdividing the mesh. (1) Surface tint/grain here on the snow
    // palette: a subtle high-frequency value-noise grain plus a slight
    // warm-white shift. (2) A procedural normal-map kick further down,
    // applied to the shading normal so the lighting reads as if the
    // surface had been displaced into low pillows. Both are gated on
    // (1-Packed) and SnowDepth so groomed cells and thin aprons stay
    // smooth.
    // Debris suppresses powder pillows — it has its own coarser roughness below.
    float powderness = (1.0 - packed) * smoothstep(0.0, 0.5, effDepth) * (1.0 - isDebris);
    if (powderness > 0.1 && pxM < 0.8) {
        float pgrain = valueNoise(vWorldPos.xz / 0.8) - 0.5;
        snow += vec3(pgrain * 0.04) * powderness;
        snow  = mix(snow, snow * vec3(1.02, 1.00, 0.97), 0.20 * powderness);
    }

    // Snowness from depth — 5 cm fully buries the ground (matches the
    // packed-apron SnowDepth so building / lift aprons read as snow,
    // not dirt). Below 5 cm, partial coverage reads as patchy snow over
    // rock/dirt. The depth field already zeroes on cliffs (auto-snow
    // slope shed), so no second slope gate is needed here. Uses
    // effDepth so tree wells expose bare ground at the trunk.
    float snowness = smoothstep(0.0, 0.05, effDepth);

    // Avalanche-debris tint: warm ochre-grey from rock/soil mixed into tumbled snow.
    // Coarse grain gives the chunky, disturbed surface character.
    if (isDebris > 0.0 && snowness > 0.05) {
        float dGrain  = fbmNoise(vWorldPos.xz / 2.2) * 0.08 - 0.04;
        vec3  debrisCol = vec3(0.78, 0.74, 0.66);
        snow = mix(snow, snow * debrisCol + vec3(dGrain), isDebris * 0.65);
    }

    vec3  base     = mix(ground, snow, snowness);

    // Map-edge skirt: a cross-section of banded rock, darker with depth.
    if (vInstabilityScore < -0.5) {
        float depthBelow = clamp((uTerrainMaxY - vWorldPos.y) / max(uTerrainMaxY - uTerrainMinY + 50.0, 1.0), 0.0, 1.0);
        float n      = valueNoise(vec2(vWorldPos.x + vWorldPos.z, vWorldPos.y) / 3.0);
        float y      = vWorldPos.y + n * 2.0;
        float strata = 0.5 + 0.3 * sin(y * 0.45) + 0.2 * sin(y * 1.7 + 1.3);
        vec3  rockA  = vec3(0.62, 0.58, 0.53);
        vec3  rockB  = vec3(0.44, 0.42, 0.40);
        base = mix(rockB, rockA, strata);
        base *= mix(1.0, 0.8, depthBelow);
    }

    // Procedural normal-map for sub-cell surface character. The terrain
    // mesh is one quad per 5 m cell; details finer than that — powder
    // pillows and mogul bumps — are added here as per-fragment normal
    // perturbations rather than as actual VS displacement. Each term
    // evaluates the same value-noise function used to drive the previous
    // (now-reverted) geometric experiment, takes a finite-difference
    // gradient in world space, and contributes a horizontal kick to N
    // proportional to the displacement-equivalent amplitude. The
    // lighting then reads as if the surface had been pushed up by
    // ~10 cm (full powder at Packed=0.2) or up to ~80 cm (MogulSize=1)
    // — without paying for mesh subdivision or a CPU mirror in
    // VisualElevationAt. Macro shape (N, slope, cliffness) keeps the
    // un-perturbed normal so rocky-ness and the cliff-vs-snow blend
    // aren't fooled by sub-cell roughness; only the lighting and
    // specular calculations see Nshading.
    // Corduroy geometry, outside any branch so derivatives stay defined.
    // Ridges sit at fixed steps of the run's lateral coordinate, stored
    // as the angle φ (one turn per 40 m), so they follow the lanes and
    // carry across seams. 160φ turns a whole number of times per wrap of
    // atan, so the ridge phase never jumps.
    vec2  cordV      = gm.gb * 2.0 - 1.0;
    float cordPhase  = atan(cordV.y, cordV.x) * 160.0;
    float cordStripe = sin(cordPhase);
    // Across-lane direction: the world-space gradient of φ, from its
    // screen derivatives (taken on cos/sin, which don't wrap).
    float cordInvL2  = 1.0 / max(dot(cordV, cordV), 1e-4);
    float dPhiX      = (cordV.x * dFdx(cordV.y) - cordV.y * dFdx(cordV.x)) * cordInvL2;
    float dPhiY      = (cordV.x * dFdy(cordV.y) - cordV.y * dFdy(cordV.x)) * cordInvL2;
    vec2  dWx        = dFdx(vWorldPos.xz);
    vec2  dWy        = dFdy(vWorldPos.xz);
    float cordDet    = dWx.x * dWy.y - dWx.y * dWy.x;
    vec2  cordGrad   = vec2(dPhiX * dWy.y - dPhiY * dWx.y, dWx.x * dPhiY - dWy.x * dPhiX) / (abs(cordDet) > 1e-8 ? cordDet : 1e-8);
    vec2  cordAcross = cordGrad * inversesqrt(max(dot(cordGrad, cordGrad), 1e-8));
    // Overlapping swaths dip the stamp to ~0.75 between lanes: the seam.
    // Where lanes planned apart meet, φ disagrees: the blended vector
    // shortens.
    float cordMis    = 1.0 - smoothstep(0.6, 0.85, length(cordV));
    float seam       = max(smoothstep(0.62, 0.72, stamp) * (1.0 - smoothstep(0.82, 0.97, stamp)),
                           cordMis * smoothstep(0.6, 0.9, stamp));
    // Fade once a ridge spans too few pixels to resolve.
    float cordAA     = 1.0 - smoothstep(1.6, 4.0, 160.0 * (abs(dPhiX) + abs(dPhiY)));
    float cordAmp    = groomVis * (1.0 - track * 0.8) * (1.0 - seam * 0.6) * (1.0 - cordMis) * cordAA * smoothstep(0.6, 0.9, stamp);

    vec3 Nshading = N;
    {
        const float bumpEps = 0.5; // world-space sample offset in metres
        vec3 kick = vec3(0);
        // Powder pillows are a few metres across; gone by 2 m per pixel.
        float powderK = powderness * (1.0 - smoothstep(1.0, 2.0, pxM));
        if (powderK > 0.05) {
            vec2 off = vec2(17.3, 91.7); // de-correlates from the mogul phase
            vec2 g = fbmGrad(vWorldPos.xz / 5.0 + off, pxM / 5.0) / 5.0;
            const float powderAmp = 0.10; // metres — matches the prior VS disp
            kick.xz -= g * powderAmp * powderK;
        }
        // Wind drifts: broad, gentle undulation on untracked snow, tens of
        // metres across, so lighting varies across open slopes.
        float driftness = (1.0 - groomVis) * smoothstep(0.05, 0.4, effDepth) * (1.0 - isDebris);
        if (driftness > 0.05) {
            vec2 g = valueNoiseD(vWorldPos.xz / 22.0 + vec2(311.7, 47.3)).yz / 22.0;
            const float driftAmp = 0.6; // metres
            kick.xz -= g * driftAmp * driftness;
        }
        float mogulK = mogul * (1.0 - smoothstep(2.0, 4.0, pxM));
        if (mogulK > 0.01 && snowness > 0.1) {
            vec2 p0 = vWorldPos.xz;
            vec2 g = fbmGrad(p0 / 3.0, pxM / 3.0) / 3.0 * 0.6
                   + fbmGrad(p0 * 0.7, pxM * 0.7) * 0.7 * 0.4;
            const float mogulAmp = 0.8;
            kick.xz -= g * mogulAmp * mogulK;
        }
        if (well > 0.01) {
            // ∇G via offset samples — the well texture is in metres of
            // depth-loss at the trunk, so each unit of well gives ~0.5 m
            // of displacement. Sample offsets are scaled into UV space.
            vec2 uvEps = vec2(bumpEps, 0) / uWorldSize;
            vec2 uvEpz = vec2(0, bumpEps) / uWorldSize;
            vec2 uv = vWorldPos.xz / uWorldSize;
            float g0 = well;
            float gx = texture(uSnowSurface, uv + uvEps).g;
            float gz = texture(uSnowSurface, uv + uvEpz).g;
            const float wellAmp = 0.5; // metres of displacement at trunk
            kick.x += (gx - g0) / bumpEps * wellAmp;
            kick.z += (gz - g0) / bumpEps * wellAmp;
        }
        if (track > 0.01) {
            // Skier-track ∇R — kick the normal along the carve direction
            // so the lane reads as a shallow groove. Sampled by texture
            // offset, same pattern as the well kick.
            vec2 uvEps = vec2(bumpEps, 0) / uWorldSize;
            vec2 uvEpz = vec2(0, bumpEps) / uWorldSize;
            vec2 uv = vWorldPos.xz / uWorldSize;
            float r0 = track;
            float rx = smoothstep(0.0, 0.5, texture(uSnowSurface, uv + uvEps).r);
            float rz = smoothstep(0.0, 0.5, texture(uSnowSurface, uv + uvEpz).r);
            const float trackAmp = 0.08; // metres — shallower than wells
            kick.x += (rx - r0) / bumpEps * trackAmp;
            kick.z += (rz - r0) / bumpEps * trackAmp;
        }
        // Debris surface roughness — coarser than moguls, represents tumbled chunks.
        // Fires instead of the powder kick (powderness is zeroed on debris cells).
        if (isDebris > 0.0 && snowness > 0.1) {
            vec2 doff = vec2(53.1, 17.8); // de-correlates from powder/mogul phases
            vec2 g = fbmGrad(vWorldPos.xz / 2.0 + doff, pxM / 2.0) / 2.0;
            const float debrisAmp = 0.30; // metres — 3× powder, reads as chunky rubble
            kick.xz -= g * debrisAmp * isDebris;
        }
        // Corduroy: ridges run along the cat's heading, about 25 cm apart
        // (exaggerated so they read from the game camera), and are lit as
        // bumps so they catch a low sun. Fresh skier tracks flatten them.
        if (cordAmp > 0.01) {
            kick.xz -= cordAcross * cos(cordPhase) * 0.35 * cordAmp;
        }
        Nshading = normalize(N + kick);
    }

    // Wrap lighting — soft terminator that hints at sub-surface scatter on snow.
    vec3  L    = uSunDir;
    const float wrap = 0.25;
    float ndl  = dot(Nshading, L);
    float diff = clamp((ndl + wrap) / (1.0 + wrap), 0.0, 1.0);
    float sunVis = keyLightVisibility(vWorldPos, N); // 0 behind ridges and in object shadows
    diff *= sunVis;

    // Cool-shadow / warm-highlight tint, applied to snow surfaces only.
    vec3 cool    = vec3(0.92, 0.95, 1.00);
    vec3 warm    = vec3(1.00, 0.95, 0.85);
    vec3 tint    = mix(cool, warm, diff);
    vec3 shaded  = base * mix(vec3(1.0), tint, snowness);

    // Baked AO shades only the sky fill (valleys and cliff bases see less
    // sky); direct sun is already handled by sunVis.
    vec3 lit = shaded * (fillLight(Nshading) * vAO + 0.85 * diff * uSunColor);
    if (uLampCount > 0) {
        lit += base * lampLight(vWorldPos, Nshading) * vAO;
    }

    // Groomed snow: the cool tint and fine grain of packed corduroy, a
    // darker line along seams between passes, and the swath edge — a
    // bright powder lip outside, a cooler scrape line inside. The ridges
    // themselves are in the normal kick above.
    if (groomVis > 0.01 && snowness > 0.1) {
        lit *= 1.0 + cordStripe * 0.05 * cordAmp;
        float grain = valueNoise(vWorldPos.xz / 1.5) - 0.5;
        lit *= 1.0 + grain * 0.04 * groomVis;
        lit  = mix(lit, lit * vec3(0.95, 0.97, 1.02), 0.15 * groomVis);
        lit *= 1.0 + 0.04 * groomVis;
        lit *= 1.0 - 0.08 * seam * groomVis;
    }
    float wear = smoothstep(0.1, 0.6, grooming);
    float edge = clamp(1.0 - abs(stamp - 0.35) * 3.5, 0.0, 1.0) * wear;
    if (edge > 0.01 && snowness > 0.1) {
        // Only the rim of the groomed area has an edge: gaps between lane
        // ends inside it are groomed all around.
        vec2 o = vec2(3.0, 0.0) / uWorldSize;
        vec2 uv = vWorldPos.xz / uWorldSize;
        float around = min(min(textureLod(uGroomMap, uv + o.xy, 0.0).r, textureLod(uGroomMap, uv - o.xy, 0.0).r),
                           min(textureLod(uGroomMap, uv + o.yx, 0.0).r, textureLod(uGroomMap, uv - o.yx, 0.0).r));
        edge *= 1.0 - smoothstep(0.3, 0.7, around);
    }
    if (edge > 0.01 && snowness > 0.1) {
        if (stamp < 0.35) {
            lit = mix(lit, vec3(1.02, 1.02, 1.00) * sceneLight(), edge * 0.30);
        } else {
            lit = mix(lit, lit * vec3(0.80, 0.86, 0.95), edge * 0.45);
        }
    }

    // Sparkle: every ~25 cm cell is an ice crystal facet tilted at random
    // off the surface. It glints only when that facet mirrors the sun into
    // the camera, so glints twinkle as the view or the sun moves and hold
    // still otherwise. Snow-only. Ice boosts specular intensity.
    if (snowness > 0.0) {
        vec3  V    = normalize(uCameraPos - vWorldPos);
        vec3  H    = normalize(L + V);
        // Facets are 25 cm; past ~1.5 m per pixel they'd only shimmer.
        float sparkleK = 1.0 - smoothstep(0.75, 1.5, pxM);
        if (sparkleK > 0.0 && sunVis > 0.0) {
            vec3  cell = floor(vWorldPos * 4.0);
            vec3  tilt = vec3(hash3(cell), hash3(cell + 17.0), hash3(cell + 41.0)) * 2.0 - 1.0;
            vec3  facet = normalize(Nshading + tilt * 0.6);
            float glint = pow(max(dot(facet, H), 0.0), 600.0);
            float sparse = step(0.80 - ice * 0.15, hash3(cell + 73.0));
            float spec = glint * sparse * dot(uSunColor, vec3(1.0 / 3.0)) * sunVis * sparkleK;
            // Debris doesn't sparkle — dirty rock/soil mixture kills specular glint.
            lit += spec * snowness * (1.0 + ice * 4.0) * vec3(2.4, 2.3, 2.1) * (1.0 - isDebris);
        }

        // Ice broad specular: a wider lobe than the sparkle, no per-cell gate.
        // Reads as a sheen across icy slopes — distinct from the rough-snow
        // sparkle.
        if (ice > 0.0) {
            float broadSpec = pow(max(dot(Nshading, H), 0.0), 32.0) * dot(uSunColor, vec3(1.0 / 3.0)) * sunVis;
            lit += broadSpec * snowness * ice * 0.45 * vec3(0.92, 0.96, 1.10) * (1.0 - isDebris);
        }
    }

    fragColor = vec4(toneMap(applyHaze(lit, vWorldPos)), 1.0);

    // ── View overlays ────────────────────────────────────────────────────────
    // uOverlayMode is a bitmask (see render.Overlay* constants). Each
    // enabled overlay alpha-blends its colour over the base shading, in
    // order, so several can stack — slope tint + corduroy lines + ice
    // heatmap all read at once, for example. Snow-state overlays only
    // fire on snowy surfaces (snowness > 0) so rocky cliffs stay
    // unmarked.
    //
    // The contour overlay still uses a high-contrast dark line because
    // mixing it with the heatmaps would wash it out — it's the one
    // overlay that wants to draw on TOP of all the others, so it's
    // applied last.

    // Slope debug — paints the whole slope, so apply early (other
    // overlays will mix over it).
    if ((uOverlayMode & 2) != 0) {
        vec3 slopeCol = mix(vec3(0.88, 0.15, 0.10),                                  // red, steep
                            mix(vec3(0.93, 0.80, 0.08), vec3(0.15, 0.72, 0.20),      // yellow → green
                                smoothstep(0.940, 0.975, slope)),
                            smoothstep(0.883, 0.940, slope));
        fragColor.rgb = mix(fragColor.rgb, slopeCol, 0.65);
    }

    // Snow quality overlay — encodes depth, surface condition, and layer
    // history in one view. Depth drives the base blue scale; grooming shifts
    // toward green; ice shifts toward silver. The combined colour gives a
    // quick read of where the good snow is without toggling five overlays.
    if ((uOverlayMode & 4) != 0) {
        float d = clamp(vSnowDepth / 5.0, 0.0, 1.0);
        float snowPresent = min(d * 4.0, 1.0); // fade effects on bare ground
        // Base: light cyan (shallow) → deep navy (deep powder).
        vec3 col = mix(vec3(0.85, 0.94, 1.00), vec3(0.10, 0.18, 0.45), d);
        // Grooming shift: pull toward bright green on freshly groomed snow.
        col = mix(col, vec3(0.20, 0.90, 0.45), grooming * 0.55 * snowPresent);
        // Ice shift: pull toward silver as surface ice rises.
        col = mix(col, vec3(0.82, 0.88, 0.95), ice * 0.70 * snowPresent);
        fragColor.rgb = mix(fragColor.rgb, col, 0.65);
    }

    // Grooming heatmap — bright green where corduroy lives. Heavy
    // alpha so a single groomed cell stands out next to its neighbours
    // when the player is scrubbing through the overlay set. Ungroomed
    // cells stay a muted slate so the contrast with green is clean.
    if ((uOverlayMode & 8) != 0 && snowness > 0.0) {
        vec3 col = mix(vec3(0.18, 0.20, 0.24), vec3(0.20, 0.95, 0.45), grooming);
        fragColor.rgb = mix(fragColor.rgb, col, 0.80 * snowness);
    }

    // Mogul heatmap — neutral → magenta as moguls rise.
    if ((uOverlayMode & 64) != 0 && snowness > 0.0) {
        vec3 col = mix(vec3(0.30, 0.30, 0.36), vec3(0.95, 0.30, 0.75), mogul);
        fragColor.rgb = mix(fragColor.rgb, col, 0.55 * snowness);
    }

    // Avalanche risk — driven directly by Cell.InstabilityScore() passed from CPU.
    // Score 0 = stable, 1.0 = at the natural release threshold (red).
    // 0 → green, 0.5 → yellow, 1.0+ → red.
    // Scores above 1.0 are clamped to red; below 1.0 the cell cannot
    // naturally release but shows the loading trend.
    if ((uOverlayMode & 16) != 0) {
        float risk    = clamp(vInstabilityScore, 0.0, 1.0);
        vec3 safe     = vec3(0.18, 0.78, 0.20);
        vec3 moderate = vec3(0.93, 0.80, 0.08);
        vec3 danger   = vec3(0.88, 0.15, 0.10);
        // green→yellow over [0, 0.5], yellow→red over [0.5, 1.0]
        float t2 = clamp(risk * 2.0, 0.0, 1.0);
        float t3 = clamp(risk * 2.0 - 1.0, 0.0, 1.0);
        vec3 riskCol  = mix(safe, mix(moderate, danger, t3), t2);
        fragColor.rgb = mix(fragColor.rgb, riskCol, 0.70);
    }

    // Bump-normal debug — render Nshading directly as RGB using the
    // standard normal-map convention (xyz remapped from [-1,1] → [0,1]).
    // Flat surface reads as (~0, ~1, ~0) → green; sloped terrain shifts
    // green toward red/blue with the fall direction; per-fragment bump
    // perturbations show as red/blue mottling on top of the green.
    // Replaces the regular shading (not blended) so the normal map is
    // legible without other overlays bleeding through. Bound to `B`.
    if ((uOverlayMode & 128) != 0) {
        fragColor.rgb = Nshading * 0.5 + 0.5;
    }

    // Surface-detail debug — paint the raw uSnowSurface texture so the
    // CPU→GPU pipeline is visible from the testbed. R=tracks, G=tree
    // wells, B=groom edges. Replaces base shading like the bump-normal
    // overlay so the channels are legible on their own. Bound to `N`.
    if ((uOverlayMode & 256) != 0) {
        fragColor.rgb = surf.rgb;
    }

    // Contour lines drawn last so they remain readable through any
    // heatmap underneath.
    if ((uOverlayMode & 1) != 0) {
        const float contourInterval = 10.0;
        float elevMod = mod(vSmoothY, contourInterval);
        float fw      = fwidth(vSmoothY);
        float line    = 1.0 - smoothstep(fw, fw * 3.0,
                                         min(elevMod, contourInterval - elevMod));
        fragColor.rgb = mix(fragColor.rgb, vec3(0.05, 0.05, 0.10), line * 0.85);
    }

    // Cell overlay (trail difficulty colours, grooming routes). One texel
    // per cell; linear filtering feathers the edges between painted and
    // unpainted cells. UV maps 0..1 across the full terrain in metres.
    {
        vec4 ov = texture(uCellOverlay, vWorldPos.xz / uWorldSize);
        fragColor.rgb = mix(fragColor.rgb, ov.rgb, ov.a);
    }

    // Brush ring — yellow, used by the glade tool.
    if (uBrushRadius > 0.0) {
        float d    = length(vWorldPos.xz - uBrushCenter);
        float ring = abs(d - uBrushRadius);
        float t    = 1.0 - clamp(ring / 5.0, 0.0, 1.0);
        fragColor  = mix(fragColor, vec4(1.0, 1.0, 0.3, 1.0), t * 0.85);
        if (d < uBrushRadius - 5.0) {
            fragColor = mix(fragColor, vec4(1.0, 1.0, 0.5, 1.0), 0.12);
        }
    }

    // Range ring — blue/white, used by the snow gun radius indicator.
    if (uRangeRadius > 0.0) {
        float d    = length(vWorldPos.xz - uRangeCenter);
        float ring = abs(d - uRangeRadius);
        float t    = 1.0 - clamp(ring / 3.0, 0.0, 1.0);
        fragColor  = mix(fragColor, vec4(0.55, 0.85, 1.0, 1.0), t * 0.9);
        if (d < uRangeRadius - 3.0) {
            fragColor = mix(fragColor, vec4(0.6, 0.9, 1.0, 1.0), 0.08);
        }
    }
}
