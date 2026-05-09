#version 410 core

in vec3 vColor;
flat in vec3 vNormal;
in vec3 vSmoothNormal;
in vec3 vWorldPos;
in float vSmoothY;

uniform vec2  uBrushCenter;
uniform float uBrushRadius;
uniform int   uOverlayMode; // 0 = normal, 1 = slope + contour
uniform vec3  uEyeDir;      // unit view direction (orthographic, scene-wide constant)

uniform sampler2D uSnowDiff;
uniform sampler2D uSnowNorm;
uniform sampler2D uSnowRough;
uniform sampler2D uRockDiff;
uniform sampler2D uRockNorm;
uniform sampler2D uRockRough;

out vec4 fragColor;

// lighting.glsl is prepended by shader loader

// Tile sizes in metres — texture repeats every N world units. Larger
// values trade per-fragment detail for less obvious tiling at the camera
// distance. The diffuse path additionally multi-scale-blends the snow
// sample so its repetition is broken up further.
const float SNOW_TILE = 12.0;
const float ROCK_TILE = 14.0;

// Slope thresholds (cos of angle from vertical) for material blend.
// Below SNOW_FLOOR (≈ 39°) it's pure rock; above SNOW_CEIL (≈ 23°) it's
// pure snow. Decoupling this from the triplanar sampling weights lets
// rock show up on moderate slopes instead of only on near-vertical
// faces.
const float SNOW_FLOOR = 0.78;
const float SNOW_CEIL  = 0.92;

float hash(vec3 p) {
    p = fract(p * vec3(0.1031, 0.1030, 0.0973));
    p += dot(p, p.yxz + 33.33);
    return fract((p.x + p.y) * p.z);
}

float noise(vec3 p) {
    vec3 i = floor(p);
    vec3 f = fract(p);
    f = f * f * (3.0 - 2.0 * f);
    return mix(
        mix(mix(hash(i),               hash(i + vec3(1,0,0)), f.x),
            mix(hash(i + vec3(0,1,0)), hash(i + vec3(1,1,0)), f.x), f.y),
        mix(mix(hash(i + vec3(0,0,1)), hash(i + vec3(1,0,1)), f.x),
            mix(hash(i + vec3(0,1,1)), hash(i + vec3(1,1,1)), f.x), f.y),
        f.z);
}

vec3 triWeights(vec3 n) {
    vec3 w = max(abs(n) - 0.2, 0.0);
    w = pow(w, vec3(4.0));
    return w / (w.x + w.y + w.z + 0.001);
}

float triDetail(vec3 pos, float scale) {
    vec3 w = triWeights(vSmoothNormal);
    return noise(vec3(pos.y, pos.z, pos.x) * scale) * w.x
         + noise(vec3(pos.x, pos.z, pos.y) * scale) * w.y
         + noise(vec3(pos.x, pos.y, pos.z) * scale) * w.z;
}

// Multi-scale sample: combines two samples at different scales/offsets,
// blended by a low-frequency variation. Hides the obvious 1:1 tiling
// pattern at the cost of one extra texture fetch.
vec3 multiScaleSample(sampler2D tex, vec2 uv, float variance) {
    vec3 a = texture(tex, uv).rgb;
    vec3 b = texture(tex, uv * 0.37 + vec2(0.13, 0.79)).rgb;
    return mix(a, b, smoothstep(0.35, 0.65, variance));
}

void main() {
    vec3 nSmooth = normalize(vSmoothNormal);
    vec3 triW = triWeights(nSmooth);

    // Triplanar UVs.
    vec2 uvX      = vWorldPos.zy / ROCK_TILE;
    vec2 uvZ      = vWorldPos.xy / ROCK_TILE;
    vec2 uvY_snow = vWorldPos.xz / SNOW_TILE;
    vec2 uvY_rock = vWorldPos.xz / ROCK_TILE;

    // Material decision: how snow-y this fragment is, based purely on
    // the smooth normal's verticality. Independent of the triplanar
    // weights so we can keep accurate side-plane sampling on cliffs
    // while still showing rock on moderate slopes.
    float snowness = smoothstep(SNOW_FLOOR, SNOW_CEIL, nSmooth.y);

    // Low-frequency variation noise (~25 m period) — drives the snow
    // multi-scale blend so adjacent regions pull from different parts
    // of the texture instead of all aligning to the same tile.
    float variance = noise(vWorldPos * vec3(0.04));

    // ── Diffuse ────────────────────────────────────────────────────────
    vec3 snowDiff = multiScaleSample(uSnowDiff, uvY_snow, variance);

    vec3 rockDX = texture(uRockDiff, uvX).rgb;
    vec3 rockDY = texture(uRockDiff, uvY_rock).rgb;
    vec3 rockDZ = texture(uRockDiff, uvZ).rgb;
    vec3 rockDiff = rockDX * triW.x + rockDY * triW.y + rockDZ * triW.z;

    vec3 baseColor = mix(rockDiff, snowDiff, snowness);

    // ── Normal map (whiteout-style triplanar swizzle, then rock/snow) ──
    // Tangent-space samples are converted to world space by mapping each
    // plane's "up" (ts.z) onto its world axis.
    vec3 nXts = texture(uRockNorm, uvX).xyz * 2.0 - 1.0;
    vec3 nZts = texture(uRockNorm, uvZ).xyz * 2.0 - 1.0;
    vec3 nYr  = texture(uRockNorm, uvY_rock).xyz * 2.0 - 1.0;
    vec3 nYs  = texture(uSnowNorm, uvY_snow).xyz * 2.0 - 1.0;

    vec3 nXws    = vec3(nXts.z, nXts.y, nXts.x);
    vec3 nZws    = vec3(nZts.x, nZts.y, nZts.z);
    vec3 nYrWs   = vec3(nYr.x,  nYr.z,  nYr.y);
    vec3 nYsWs   = vec3(nYs.x,  nYs.z,  nYs.y);

    vec3 rockNorm = nXws * triW.x + nYrWs * triW.y + nZws * triW.z;
    vec3 perturbed = normalize(mix(rockNorm, nYsWs, snowness));

    // ── Roughness ──────────────────────────────────────────────────────
    float rrX = texture(uRockRough, uvX).r;
    float rrY = texture(uRockRough, uvY_rock).r;
    float rrZ = texture(uRockRough, uvZ).r;
    float rockRough = rrX * triW.x + rrY * triW.y + rrZ * triW.z;
    float snowRough = texture(uSnowRough, uvY_snow).r;
    float roughness = clamp(mix(rockRough, snowRough, snowness), 0.0, 1.0);

    // ── Macro detail noise — stronger amplitude further hides tiling. ──
    float detail = triDetail(vWorldPos, 0.018) * 0.6
                 + triDetail(vWorldPos, 0.45)  * 0.4;
    baseColor *= mix(0.86, 1.14, detail);

    // ── Lighting: shared diffuse + ambient, plus Blinn-Phong specular
    //              shaped by the roughness map. ────────────────────────
    vec3 lit = computeLighting(perturbed, baseColor);

    vec3 lightDir = normalize(vec3(0.6, 1.0, 0.4));
    vec3 halfDir  = normalize(lightDir + uEyeDir);
    float shininess  = mix(64.0, 4.0, roughness);
    float specPower  = pow(max(dot(perturbed, halfDir), 0.0), shininess);
    float specWeight = mix(0.45, 0.04, roughness);
    lit += specPower * specWeight * vec3(1.0);

    fragColor = vec4(lit, 1.0);

    if (uOverlayMode == 1) {
        // Slope colour: green (gentle) → yellow (moderate) → red (steep)
        // Thresholds in terms of normal.y = cos(angle):
        //   green  < 15°  → normal.y > 0.966
        //   yellow 15-28° → normal.y 0.883–0.966
        //   red    > 28°  → normal.y < 0.883
        float n = clamp(vSmoothNormal.y, 0.0, 1.0);
        vec3 slopeColor = mix(vec3(0.88, 0.15, 0.10),   // red
                              vec3(0.93, 0.80, 0.08),    // yellow
                              smoothstep(0.883, 0.920, n));
        slopeColor      = mix(slopeColor,
                              vec3(0.15, 0.72, 0.20),    // green
                              smoothstep(0.940, 0.975, n));
        fragColor.rgb = mix(fragColor.rgb, slopeColor, 0.72);

        // Contour lines every 50 m; fwidth keeps line width constant on screen.
        // vSmoothY is a low-pass-filtered elevation (built CPU-side) — using it
        // here instead of vWorldPos.y removes the per-vertex jitter and the
        // triangle-grid stepping that otherwise zig-zag the contour lines.
        float elevMod = mod(vSmoothY, 50.0);
        float fw      = fwidth(vSmoothY);
        float line    = 1.0 - smoothstep(fw, fw * 3.0,
                                         min(elevMod, 50.0 - elevMod));
        fragColor.rgb = mix(fragColor.rgb, vec3(0.05, 0.05, 0.10), line * 0.88);
    }

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
