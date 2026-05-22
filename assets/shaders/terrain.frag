#version 410 core

flat in vec3  vNormal;
in vec3  vWorldPos;
in float vSmoothY;
in float vAO;
in vec4  vSnow;        // (Grooming, Packed, Ice, MogulSize)
in float vSnowDepth;   // SnowDepth in metres
in vec3  vSmoothNormal; // per-corner smoothed normal, interpolated across triangles

uniform vec2  uBrushCenter;
uniform float uBrushRadius;
uniform int   uOverlayMode;     // 0 = off, 1 = contour, 2 = slope debug
uniform vec3  uCameraPos;
uniform float uTime;
uniform float uTerrainMinY;
uniform float uTerrainMaxY;

// Sub-cell surface-detail texture, mirrored from world.SurfaceDetail.
// Resolution = (Width*5, Height*5) pixels at 1 m per pixel. Channels:
//   R = skier track intensity (decays in sim time)
//   G = tree-well depth      (persistent until tree edits)
//   B = groom-edge mask      (derived from per-cell Grooming)
//   A = reserved
// uWorldSize is the terrain extent in metres = cells × 5, so
// vWorldPos.xz / uWorldSize is the texture's UV.
uniform sampler2D uSnowSurface;
uniform vec2      uWorldSize;

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

void main() {
    float grooming = clamp(vSnow.x, 0.0, 1.0);
    float packed   = clamp(vSnow.y, 0.0, 1.0);
    float ice      = clamp(vSnow.z, 0.0, 1.0);
    float mogul    = clamp(vSnow.w, 0.0, 1.0);

    // Surface-detail sample — sub-cell features rendered from the
    // 1 m-resolution texture written by the simulation. R = skier
    // tracks (step 4), G = tree-well depth, B = groom-edge (step 3).
    vec4 surf = texture(uSnowSurface, vWorldPos.xz / uWorldSize);
    // Normalise raw R (additive splats, 0.25 per pass) so one pass reads
    // as half-intensity and two saturate. This caps the crossing peak at
    // the same visual level as a single pass, fixing the blobby gradient
    // artifact where ∇R → 0 at a local maximum.
    // Tracks are suppressed on groomed snow — fresh corduroy visually
    // overpowers light ski marks, so scale them down proportionally.
    float track = smoothstep(0.0, 0.5, surf.r) * (1.0 - grooming * 0.85);
    float well  = surf.g;
    float edge  = surf.b;

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

    // Snow palette — the previous topographic gradient. Packed reads
    // slightly bluer / ~5 % darker than fresh powder; modulate before
    // mixing with ground so packed never leaks onto bare rock.
    vec3 lowFlat   = vec3(0.82, 0.85, 0.92);             // cool shadow tint
    vec3 midPowder = vec3(0.92, 0.94, 0.97);
    vec3 highWhite = vec3(0.99, 0.99, 1.00);
    vec3 snow      = mix(lowFlat, midPowder, smoothstep(0.20, 0.55, h));
         snow      = mix(snow,    highWhite, smoothstep(0.65, 0.95, h));
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
    float powderness = (1.0 - packed) * smoothstep(0.0, 0.5, effDepth);
    if (powderness > 0.1) {
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
    vec3  base     = mix(ground, snow, snowness);

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
    vec3 Nshading = N;
    {
        const float bumpEps = 0.5; // world-space sample offset in metres
        vec3 kick = vec3(0);
        if (powderness > 0.05) {
            vec2 off = vec2(17.3, 91.7); // de-correlates from the mogul phase
            float h0 = fbmNoise(vWorldPos.xz / 5.0 + off);
            float hx = fbmNoise((vWorldPos.xz + vec2(bumpEps, 0)) / 5.0 + off);
            float hz = fbmNoise((vWorldPos.xz + vec2(0, bumpEps)) / 5.0 + off);
            const float powderAmp = 0.10; // metres — matches the prior VS disp
            kick.x -= (hx - h0) / bumpEps * powderAmp * powderness;
            kick.z -= (hz - h0) / bumpEps * powderAmp * powderness;
        }
        if (mogul > 0.01 && snowness > 0.1) {
            vec2 p0 = vWorldPos.xz;
            vec2 px = p0 + vec2(bumpEps, 0);
            vec2 pz = p0 + vec2(0, bumpEps);
            float h0 = fbmNoise(p0 / 3.0) * 0.6 + fbmNoise(p0 * 2.1 / 3.0) * 0.4;
            float hx = fbmNoise(px / 3.0) * 0.6 + fbmNoise(px * 2.1 / 3.0) * 0.4;
            float hz = fbmNoise(pz / 3.0) * 0.6 + fbmNoise(pz * 2.1 / 3.0) * 0.4;
            const float mogulAmp = 0.8;
            kick.x -= (hx - h0) / bumpEps * mogulAmp * mogul;
            kick.z -= (hz - h0) / bumpEps * mogulAmp * mogul;
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
        Nshading = normalize(N + kick);
    }

    // Wrap lighting — soft terminator that hints at sub-surface scatter on snow.
    vec3  L    = normalize(vec3(0.6, 1.0, 0.4));
    const float wrap = 0.5;
    float ndl  = dot(Nshading, L);
    float diff = clamp((ndl + wrap) / (1.0 + wrap), 0.0, 1.0);

    // Cool-shadow / warm-highlight tint, applied to snow surfaces only.
    vec3 cool    = vec3(0.55, 0.65, 0.85);
    vec3 warm    = vec3(1.00, 0.95, 0.85);
    vec3 tint    = mix(cool, warm, diff);
    vec3 shaded  = base * mix(vec3(1.0), tint, snowness);

    // Ambient + diffuse, multiplied by baked AO to deepen valleys / cliff bases.
    vec3 lit = shaded * (0.25 + 0.85 * diff) * vAO;

    // Groomed snow visual. Three layered cues:
    //   1. Corduroy stripes — a sine pattern projected along the
    //      contour direction (perpendicular to the local fall line).
    //      Direction comes from smoothN.xz, the heavily filtered
    //      CPU-side normal (5-tap binomial × 2 passes before vert
    //      shader); its horizontal projection is the smoothed fall
    //      direction. Per-fragment interpolation produces continuous
    //      contour-following stripes that gently curve with the
    //      terrain, which is what real corduroy looks like on a
    //      curving piste. Gated on slope so flat aprons don't pick
    //      up garbage from noisy near-zero gradients, and faded in
    //      over the slope threshold so groomed flats blend cleanly
    //      into pitched groomed runs.
    //   2. A subtle low-amplitude 2D value-noise grain — fine snow
    //      texture between the cords.
    //   3. A slight cool tint — packed-and-smoothed snow reads
    //      bluer than fresh powder.
    if (grooming > 0.01 && snowness > 0.1) {
        vec2 horizN = N.xz;
        float horizLen = length(horizN);
        if (horizLen > 0.05) {
            vec2 fallDir    = horizN / horizLen;
            vec2 contourDir = vec2(-fallDir.y, fallDir.x);
            const float stripesPerMeter = 4.0;
            float phase  = dot(vWorldPos.xz, contourDir) * stripesPerMeter * 6.2831853;
            float stripe = sin(phase);
            // Anti-alias: fade the stripe out when one period spans
            // less than a few pixels on screen. fwidth(phase) gives the
            // total phase change across a 2×2 pixel quad, so we fade
            // once that exceeds ~π (Nyquist) toward ~3π where the
            // moiré would otherwise dominate.
            float phaseFW = fwidth(phase);
            float aaFade  = 1.0 - smoothstep(1.6, 4.0, phaseFW);
            float slopeFade = smoothstep(0.05, 0.15, horizLen);
            // Stripe amplitude is the strongest grooming signal — a
            // single groomed cell only reads ~25 % grooming at its
            // corners after the 4-cell average, so the base amplitude
            // has to be loud enough that 25 % × amp is still visible.
            lit *= 1.0 + stripe * 0.15 * grooming * slopeFade * aaFade;
        }
        float grain = valueNoise(vWorldPos.xz / 1.5) - 0.5; // ±0.5 around zero
        lit *= 1.0 + grain * 0.04 * grooming;
        // Subtle cool tint and brightness lift — the primary grooming
        // signal is the cord stripes plus the depth step revealed by
        // the divergence-driven flat-normal lighting, so colour shifts
        // stay quiet.
        lit  = mix(lit, lit * vec3(0.95, 0.97, 1.02), 0.15 * grooming);
        lit *= 1.0 + 0.04 * grooming;
    }

    // Groomed/ungroomed edge from the surface-detail B channel. The
    // mask reads non-zero in the 1 m band on either side of any
    // cell-to-cell grooming step. We split sides by the current
    // fragment's grooming: on the powder side we lift toward white
    // (the natural shoulder where un-tracked snow piles up against
    // the cat's swath), and on the groomed side we deepen toward a
    // cool grey (the compacted scrape line). Replaces the previous
    // soft 2-cell colour ramp with a crisp line; the geometric step
    // is still doing most of the depth work.
    if (edge > 0.01 && snowness > 0.1) {
        if (grooming < 0.5) {
            // Powder shoulder — bright, warm white lip.
            float lip = edge;
            lit = mix(lit, vec3(1.02, 1.02, 1.00), lip * 0.30);
        } else {
            // Groomed scrape edge — cooler, slightly darker.
            float scrape = edge;
            lit = mix(lit, lit * vec3(0.80, 0.86, 0.95), scrape * 0.45);
        }
    }

    // Per-fragment sparkle: high-freq world-cell hash gated by tight specular
    // alignment, drifting in time so cells flicker on/off without the camera
    // having to move. Snow-only. Ice boosts specular intensity dramatically.
    if (snowness > 0.0) {
        vec3  V    = normalize(uCameraPos - vWorldPos);
        vec3  H    = normalize(L + V);
        float spec = pow(max(dot(Nshading, H), 0.0), 256.0);
        vec3  cell = floor(vWorldPos * 2.0 + uTime * 0.07); // ~50 cm cells
        float gate = step(0.985 - ice * 0.05, hash3(cell));
        lit += gate * spec * snowness * (1.0 + ice * 4.0) * vec3(1.4, 1.35, 1.2);

        // Ice broad specular: a wider lobe than the sparkle, no per-cell gate.
        // Reads as a sheen across icy slopes — distinct from the rough-snow
        // sparkle.
        if (ice > 0.0) {
            float broadSpec = pow(max(dot(Nshading, H), 0.0), 32.0);
            lit += broadSpec * snowness * ice * 0.45 * vec3(0.92, 0.96, 1.10);
        }
    }

    fragColor = vec4(lit, 1.0);

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

    // Brush ring (unchanged behaviour).
    if (uBrushRadius > 0.0) {
        float d    = length(vWorldPos.xz - uBrushCenter);
        float ring = abs(d - uBrushRadius);
        float t    = 1.0 - clamp(ring / 5.0, 0.0, 1.0);
        fragColor  = mix(fragColor, vec4(1.0, 1.0, 0.3, 1.0), t * 0.85);
        if (d < uBrushRadius - 5.0) {
            fragColor = mix(fragColor, vec4(1.0, 1.0, 0.5, 1.0), 0.12);
        }
    }
}
