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

void main() {
    float grooming = clamp(vSnow.x, 0.0, 1.0);
    float packed   = clamp(vSnow.y, 0.0, 1.0);
    float ice      = clamp(vSnow.z, 0.0, 1.0);
    float mogul    = clamp(vSnow.w, 0.0, 1.0);

    // Smooth normals on gentle terrain, flat (per-triangle) normals on
    // steep slopes. The faceted shading on cliffs reads as crisp rock
    // breaks; the smooth elsewhere reads as continuous snow.
    //
    // Additionally, shift toward the flat normal wherever smoothN and
    // flatN disagree noticeably — that disagreement means the mesh has
    // a real geometric step (e.g. a groomed lane sitting lower than
    // the untracked shoulder due to SWE compression) that the CPU-side
    // smoothing has averaged away. Without this, the depth drop from
    // grooming/packing produces a smooth dip rather than the sharp
    // shadowed shelf the player should be reading the piste from.
    vec3 flatN    = normalize(vNormal);
    vec3 smoothN  = normalize(vSmoothNormal);
    float cliffness  = 1.0 - smoothstep(0.55, 0.85, smoothN.y);
    float divergence = 1.0 - dot(smoothN, flatN);
    float stepReveal = smoothstep(0.02, 0.20, divergence);
    float flatWeight = max(cliffness, stepReveal);
    vec3  N          = normalize(mix(smoothN, flatN, flatWeight));
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
    // Packed-snow tint — slightly darker and a touch cooler than fresh
    // powder. Kept subtle so the primary signal of packed/groomed snow
    // is the geometric depth step (revealed via flat-normal lighting at
    // boundaries above), not a strong colour shift. Default Packed=0.5
    // sits near zero shift; groomed Packed=1 reads ~5% darker / faintly
    // cool.
    vec3 packedTint = vec3(0.92, 0.94, 0.97);
    float packedMix = smoothstep(0.45, 1.0, packed) * 0.45;
    snow            = mix(snow, snow * packedTint, packedMix);

    // Mogul roughness — multi-octave value-noise modulating brightness.
    // Mogul wavelength ~3 m matches real spacing; a half-wavelength octave
    // adds inter-trough detail. Applied to the snow layer only; the
    // ground/snow mix below attenuates the effect on patchy / bare cells.
    if (mogul > 0.0) {
        vec2 mp = vWorldPos.xz / 3.0;
        float n = valueNoise(mp) * 0.6 + valueNoise(mp * 2.1) * 0.4;
        snow += vec3((n - 0.5) * 0.35 * mogul);
    }

    // Snowness from depth — 5 cm fully buries the ground (matches the
    // packed-apron SnowDepth so building / lift aprons read as snow,
    // not dirt). Below 5 cm, partial coverage reads as patchy snow over
    // rock/dirt. The depth field already zeroes on cliffs (auto-snow
    // slope shed), so no second slope gate is needed here.
    float snowness = smoothstep(0.0, 0.05, vSnowDepth);
    vec3  base     = mix(ground, snow, snowness);

    // Wrap lighting — soft terminator that hints at sub-surface scatter on snow.
    vec3  L    = normalize(vec3(0.6, 1.0, 0.4));
    const float wrap = 0.5;
    float ndl  = dot(N, L);
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
        vec2 horizN = smoothN.xz;
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

    // Per-fragment sparkle: high-freq world-cell hash gated by tight specular
    // alignment, drifting in time so cells flicker on/off without the camera
    // having to move. Snow-only. Ice boosts specular intensity dramatically.
    if (snowness > 0.0) {
        vec3  V    = normalize(uCameraPos - vWorldPos);
        vec3  H    = normalize(L + V);
        float spec = pow(max(dot(N, H), 0.0), 256.0);
        vec3  cell = floor(vWorldPos * 2.0 + uTime * 0.07); // ~50 cm cells
        float gate = step(0.985 - ice * 0.05, hash3(cell));
        lit += gate * spec * snowness * (1.0 + ice * 4.0) * vec3(1.4, 1.35, 1.2);

        // Ice broad specular: a wider lobe than the sparkle, no per-cell gate.
        // Reads as a sheen across icy slopes — distinct from the rough-snow
        // sparkle.
        if (ice > 0.0) {
            float broadSpec = pow(max(dot(N, H), 0.0), 32.0);
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

    // Snow depth heatmap — light cyan at 0, deep navy at saturated.
    // Typical depths run 0–4 m; saturate at 5 m so the colour scale spans
    // the meaningful range. Drawn over bare cells too so "no snow here"
    // reads directly from the overlay instead of looking like the
    // overlay is just off.
    if ((uOverlayMode & 4) != 0) {
        float d = clamp(vSnowDepth / 5.0, 0.0, 1.0);
        vec3 col = mix(vec3(0.85, 0.94, 1.00), vec3(0.10, 0.18, 0.45), d);
        fragColor.rgb = mix(fragColor.rgb, col, 0.55);
    }

    // Grooming heatmap — bright green where corduroy lives. Heavy
    // alpha so a single groomed cell stands out next to its neighbours
    // when the player is scrubbing through the overlay set. Ungroomed
    // cells stay a muted slate so the contrast with green is clean.
    if ((uOverlayMode & 8) != 0 && snowness > 0.0) {
        vec3 col = mix(vec3(0.18, 0.20, 0.24), vec3(0.20, 0.95, 0.45), grooming);
        fragColor.rgb = mix(fragColor.rgb, col, 0.80 * snowness);
    }

    // Packed snow heatmap — cool-to-warm scale (loose powder → bulletproof).
    if ((uOverlayMode & 16) != 0 && snowness > 0.0) {
        vec3 col = mix(vec3(0.30, 0.50, 0.95), vec3(0.95, 0.55, 0.20), packed);
        fragColor.rgb = mix(fragColor.rgb, col, 0.55 * snowness);
    }

    // Ice heatmap — neutral → bright cyan as ice rises.
    if ((uOverlayMode & 32) != 0 && snowness > 0.0) {
        vec3 col = mix(vec3(0.30, 0.30, 0.36), vec3(0.30, 0.95, 1.00), ice);
        fragColor.rgb = mix(fragColor.rgb, col, 0.55 * snowness);
    }

    // Mogul heatmap — neutral → magenta as moguls rise.
    if ((uOverlayMode & 64) != 0 && snowness > 0.0) {
        vec3 col = mix(vec3(0.30, 0.30, 0.36), vec3(0.95, 0.30, 0.75), mogul);
        fragColor.rgb = mix(fragColor.rgb, col, 0.55 * snowness);
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
