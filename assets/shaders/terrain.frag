#version 410 core

flat in vec3  vNormal;
in vec3  vWorldPos;
in float vSmoothY;
in float vAO;
in vec4  vSnow;             // (Grooming, Packed, Ice, MogulSize)
in float vSnowDepth;        // SnowDepth in metres
in vec3  vSmoothNormal;     // per-corner smoothed normal, interpolated across triangles
in float vInstabilityScore; // Cell.InstabilityScore(); 0=stable, 1.0=release threshold; -1 on map-edge skirts
in float vPatch;            // the snow's broad patchiness, 0–1 (terrain.tese)
in vec2  vDrift;            // the wind drifts' slope (terrain.tese)

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
// vWorldPos.xz / uWorldSize (mogul.glsl) is the texture's UV.
// It is cut into square tiles, the layers of an array texture (a 2D
// texture's side is capped below Kirkwood's size); each layer holds
// kSurfaceTile pixels plus a one-pixel apron from its neighbours, so
// filtering never reads across a tile's edge (render/snow_surface.go).
uniform sampler2DArray uSnowSurface;
uniform vec2 uSurfaceSize;  // the whole surface in pixels
uniform int  uSurfaceTilesX; // tiles across
const int kSurfaceTile = 2048; // render.surfaceTile
uniform float uTrackClock;      // game minutes, mod 65536 (world.TrackClock plus the fraction)

// surfaceTexel is the surface's pixel p, clamped to its edge.
vec4 surfaceTexel(ivec2 p) {
    p = clamp(p, ivec2(0), ivec2(uSurfaceSize) - 1);
    ivec2 t = p / kSurfaceTile;
    return texelFetch(uSnowSurface, ivec3(p - t * kSurfaceTile + 1, t.y * uSurfaceTilesX + t.x), 0);
}

// surfaceSample is the surface filtered at uv (0..1 across the map), from
// the tile under it: its apron covers the filter's reach past the edge.
vec4 surfaceSample(vec2 uv) {
    vec2  p = clamp(uv * uSurfaceSize, vec2(0.5), uSurfaceSize - 0.5);
    ivec2 t = min(ivec2(p) / kSurfaceTile, ivec2(uSurfaceSize - 1.0) / kSurfaceTile);
    vec2  l = (p - vec2(t * kSurfaceTile) + 1.0) / float(kSurfaceTile + 2);
    return texture(uSnowSurface, vec3(l, float(t.y * uSurfaceTilesX + t.x)));
}

// Where snowcats groomed, mirrored from world.GroomMap at 1 m per texel:
//   R = groomed, GB = cat heading as (cos 2θ, sin 2θ) mapped to 0..1,
//   A = sideways offset from the pass centreline, ±2.5 m mapped to 0..1.
uniform sampler2D uGroomMap;

// Per-cell RGBA8 overlay: trails (green/blue/black) and grooming routes (cyan).
// One texel per terrain cell; linear filtering feathers cell edges.
// Alpha 0 = no overlay. UV = vWorldPos.xz / uWorldSize.
uniform sampler2D uCellOverlay;

// What the ground is made of (world.TerrainMaterial): one byte per
// 1.25 m detail sample, sample (i, j) at world (i, j) × 1.25 m. IDs
// match world.Material. uMaterialOn is 0 when the map has none.
// Sub-cell height detail (see terrain.tese). When the terrain is drawn at
// a coarse level of detail (uFragDetail = 1), its slope tilts the normal
// here, per pixel, so the lidar's relief still shades at full resolution.
uniform sampler2D uDetail;
uniform float     uDetailOn;
uniform vec2      uDetailSize;
uniform float     uFragDetail;

// Per-corner snow state (see terrain.vert): at a coarse level of detail
// it's read here, per pixel, rather than interpolated from the coarse
// triangle's corners, so patchy snow (thin under forest, deep between)
// keeps its pattern.
uniform sampler2D uCornerSnowA;
uniform sampler2D uCornerSnowB;

// cornerSnow is the snow state at world xz, bilinear between corners:
// A = (grooming, packed, ice, mogul), and the visible depth.
void cornerSnow(vec2 xz, out vec4 a, out float depth) {
    vec2  g  = xz / 5.0;
    ivec2 c  = ivec2(floor(g));
    vec2  f  = fract(g);
    ivec2 hi = textureSize(uCornerSnowA, 0) - 1;
    ivec2 c00 = clamp(c, ivec2(0), hi), c10 = clamp(c + ivec2(1, 0), ivec2(0), hi);
    ivec2 c01 = clamp(c + ivec2(0, 1), ivec2(0), hi), c11 = clamp(c + ivec2(1, 1), ivec2(0), hi);
    a = mix(mix(texelFetch(uCornerSnowA, c00, 0), texelFetch(uCornerSnowA, c10, 0), f.x),
            mix(texelFetch(uCornerSnowA, c01, 0), texelFetch(uCornerSnowA, c11, 0), f.x), f.y);
    depth = mix(mix(texelFetch(uCornerSnowB, c00, 0).r, texelFetch(uCornerSnowB, c10, 0).r, f.x),
                mix(texelFetch(uCornerSnowB, c01, 0).r, texelFetch(uCornerSnowB, c11, 0).r, f.x), f.y);
}

// debrisAt is the depth of avalanche debris at world xz (world.Cell's
// Debris), metres, bilinear between corners: uCornerSnowB's third channel.
float debrisAt(vec2 xz) {
    vec2  g  = xz / 5.0;
    ivec2 c  = ivec2(floor(g));
    vec2  f  = fract(g);
    ivec2 hi = textureSize(uCornerSnowB, 0) - 1;
    float a = texelFetch(uCornerSnowB, clamp(c, ivec2(0), hi), 0).b;
    float b = texelFetch(uCornerSnowB, clamp(c + ivec2(1, 0), ivec2(0), hi), 0).b;
    float d = texelFetch(uCornerSnowB, clamp(c + ivec2(0, 1), ivec2(0), hi), 0).b;
    float e = texelFetch(uCornerSnowB, clamp(c + ivec2(1, 1), ivec2(0), hi), 0).b;
    return mix(mix(a, b, f.x), mix(d, e, f.x), f.y);
}

uniform sampler2D uMaterial;
uniform float     uMaterialOn;
uniform vec2      uMaterialSize;

const int MAT_MEADOW = 0, MAT_DIRT = 1, MAT_SCREE = 2, MAT_ROCK = 3,
          MAT_ICE = 4, MAT_THIN_ICE = 5, MAT_OPEN_WATER = 6;

out vec4 fragColor;

#ifdef TUNNEL_MOUTHS
// Tunnel mouths (render/tunnel_mouths.go): the ground's grid slopes from
// the bed in front of a portal's face to the hill behind it, so the
// terrain isn't drawn there: in front of the face, above the bed; and
// behind it across the opening, from the bed to the opening's top, as
// far in as the portal's dark inside. A: (x, z, heading, bed's height);
// B: (face half-width, opening half-width, opening height, inside depth).
const int MAX_MOUTHS = 16;
uniform int  uMouthCount;
uniform vec4 uMouthA[MAX_MOUTHS];
uniform vec4 uMouthB[MAX_MOUTHS];

bool inMouth(vec3 p) {
    for (int i = 0; i < uMouthCount; i++) {
        vec4 a = uMouthA[i];
        vec4 b = uMouthB[i];
        vec2 dir = vec2(sin(a.z), cos(a.z));
        vec2 d = p.xz - a.xy;
        float u = dot(d, dir);
        float v = abs(dot(d, vec2(dir.y, -dir.x)));
        float above = p.y - a.w;
        if (above < 0.5 || u < -2.0 || u > b.w) continue; // -2: mouthFront
        if (u <= 0.0 && v < b.x) return true;
        if (u > 0.0 && v < b.y && above < b.z + 0.3) return true;
    }
    return false;
}
// Only the chunks with a mouth are drawn with this (TerrainMouthShader):
// a shader that can discard turns off the GPU's hidden-surface removal
// for everything it draws.
#endif

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

// Avalanche debris is a jumble of rounded blocks: a cell noise whose
// cells are the blocks. debrisBlocks gives, for p in block widths, the
// distance to the nearest block's centre (x), how much nearer that is
// than the next (y: small along the cracks between blocks), and the
// offset from the nearest centre (zw), which lights the block as a dome.
vec4 debrisBlocks(vec2 p) {
    vec2  i  = floor(p);
    float f1 = 9.0, f2 = 9.0;
    vec2  off = vec2(0.0);
    for (int y = -1; y <= 1; y++) {
        for (int x = -1; x <= 1; x++) {
            vec2  c = i + vec2(x, y);
            vec2  o = vec2(hash3(vec3(c, 3.1)), hash3(vec3(c, 7.7)));
            vec2  d = p - (c + 0.15 + 0.7 * o);
            float l = length(d);
            if (l < f1) {
                f2 = f1; f1 = l; off = d;
            } else if (l < f2) {
                f2 = l;
            }
        }
    }
    return vec4(f1, f2 - f1, off);
}

int materialAt(ivec2 p) {
    p = clamp(p, ivec2(0), ivec2(uMaterialSize) - 1);
    return int(texelFetch(uMaterial, p, 0).r * 255.0 + 0.5);
}

// isLakeMat reports whether material m is a lake's or creek's water.
bool isLakeMat(int m) { return m == MAT_ICE || m == MAT_THIN_ICE || m == MAT_OPEN_WATER; }

// shoreSnow keeps the snow at a lake's edge to its own side: the corners
// (5 m apart) carry less snow on the water than on the land, and blended
// across a corner square the edge of the snow would follow the squares
// in a sawtooth. So where the corners round xz differ, the snow there is
// blended from only the corners on xz's side of the shore, the side
// from the 1.25 m material map, and the snow's edge follows the water's.
void shoreSnow(vec2 xz, inout vec4 a, inout float depth) {
    vec2  g  = xz / 5.0;
    ivec2 c  = ivec2(floor(g));
    vec2  f  = fract(g);
    ivec2 hi = textureSize(uCornerSnowA, 0) - 1;
    ivec2 cs[4] = ivec2[4](c, c + ivec2(1, 0), c + ivec2(0, 1), c + ivec2(1, 1));
    float ws[4] = float[4]((1.0 - f.x) * (1.0 - f.y), f.x * (1.0 - f.y), (1.0 - f.x) * f.y, f.x * f.y);
    bool  lake[4];
    int   n = 0;
    for (int k = 0; k < 4; k++) {
        lake[k] = isLakeMat(materialAt(clamp(cs[k], ivec2(0), hi) * 4));
        n += lake[k] ? 1 : 0;
    }
    if (n == 0 || n == 4) {
        return;
    }
    // Which side xz is on: the four material samples round it, blended
    // and roughened a little, as the open water's edge is.
    vec2  m  = xz / 1.25;
    ivec2 p  = ivec2(floor(m));
    vec2  fm = fract(m);
    float l00 = isLakeMat(materialAt(p)) ? 1.0 : 0.0;
    float l10 = isLakeMat(materialAt(p + ivec2(1, 0))) ? 1.0 : 0.0;
    float l01 = isLakeMat(materialAt(p + ivec2(0, 1))) ? 1.0 : 0.0;
    float l11 = isLakeMat(materialAt(p + ivec2(1, 1))) ? 1.0 : 0.0;
    float lw  = mix(mix(l00, l10, fm.x), mix(l01, l11, fm.x), fm.y);
    bool  onLake = lw + 0.25 * (valueNoise(xz / 0.9) - 0.5) > 0.5;
    vec4  sa = vec4(0.0);
    float sd = 0.0, sw = 0.0;
    for (int k = 0; k < 4; k++) {
        if (lake[k] != onLake) {
            continue;
        }
        ivec2 cc = clamp(cs[k], ivec2(0), hi);
        sa += texelFetch(uCornerSnowA, cc, 0) * ws[k];
        sd += texelFetch(uCornerSnowB, cc, 0).r * ws[k];
        sw += ws[k];
    }
    if (sw > 1e-4) {
        a = sa / sw;
        depth = sd / sw;
    }
}

// Bare ground colour for a material that isn't rock; grain (0–1)
// breaks up flat fills. Rock is drawn by rockField and rockAlbedo.
vec3 materialColor(int m, vec3 meadow, float grain) {
    if (m == MAT_DIRT)  return vec3(0.40, 0.33, 0.25) * (0.90 + 0.20 * grain);
    if (m == MAT_SCREE) return vec3(0.55, 0.52, 0.48) * (0.80 + 0.35 * grain);
    if (m == MAT_ICE)        return vec3(0.30, 0.40, 0.46) * (0.92 + 0.12 * grain); // lake ice
    if (m == MAT_THIN_ICE)   return vec3(0.17, 0.23, 0.27) * (0.92 + 0.12 * grain); // dark new ice
    if (m == MAT_OPEN_WATER) return vec3(0.08, 0.15, 0.17);                           // under the water sheet (render/water.go)
    return meadow;
}

// Bare ground at wp from the material map, blending the four samples
// around it so material edges don't show the 1.25 m lattice: rgb is the
// ground that isn't rock, a how much of it is rock.
vec4 groundFromMaterial(vec3 wp, vec3 meadow) {
    vec2  g     = wp.xz / 1.25;
    ivec2 p     = ivec2(floor(g));
    vec2  f     = fract(g);
    float grain = fbmNoise(wp.xz / 1.7 + vec2(wp.y * 0.3, 0.0));
    vec4 acc = vec4(0.0);
    float wsum = 0.0;
    for (int k = 0; k < 4; k++) {
        ivec2 o  = ivec2(k & 1, k >> 1);
        float w  = (o.x == 1 ? f.x : 1.0 - f.x) * (o.y == 1 ? f.y : 1.0 - f.y);
        int   m  = materialAt(p + o);
        if (m == MAT_ROCK) {
            acc.a += w;
        } else {
            acc.rgb += materialColor(m, meadow, grain) * w;
            wsum += w;
        }
    }
    acc.rgb = wsum > 1e-4 ? acc.rgb / wsum : meadow;
    return acc;
}

// 3D value noise, so rock patterns wrap faces at any angle without the
// stretching a top-down projection gives near-vertical walls.
float valueNoise3(vec3 p) {
    vec3 i = floor(p);
    vec3 f = fract(p);
    f = f * f * (3.0 - 2.0 * f);
    float n000 = hash3(i);
    float n100 = hash3(i + vec3(1, 0, 0));
    float n010 = hash3(i + vec3(0, 1, 0));
    float n110 = hash3(i + vec3(1, 1, 0));
    float n001 = hash3(i + vec3(0, 0, 1));
    float n101 = hash3(i + vec3(1, 0, 1));
    float n011 = hash3(i + vec3(0, 1, 1));
    float n111 = hash3(i + vec3(1, 1, 1));
    return mix(mix(mix(n000, n100, f.x), mix(n010, n110, f.x), f.y),
               mix(mix(n001, n101, f.x), mix(n011, n111, f.x), f.y), f.z);
}

float fbm3(vec3 p) {
    return (valueNoise3(p) + 0.5 * valueNoise3(p * 2.03) + 0.25 * valueNoise3(p * 4.01)) / 1.75;
}

// Cellular noise in 3D: distances to the nearest and second-nearest
// jittered cell point (F1, F2), and a hash of the nearest cell.
vec3 worley3(vec3 p) {
    vec3  i  = floor(p);
    vec3  f  = fract(p);
    float d1 = 8.0, d2 = 8.0, id = 0.0;
    for (int z = -1; z <= 1; z++)
    for (int y = -1; y <= 1; y++)
    for (int x = -1; x <= 1; x++) {
        vec3  g = vec3(x, y, z);
        vec3  c = i + g;
        vec3  o = vec3(hash3(c), hash3(c + vec3(17.1, 5.3, 9.7)), hash3(c + vec3(3.1, 41.7, 23.9)));
        vec3  r = g + o - f;
        float d = dot(r, r);
        if (d < d1) { d2 = d1; d1 = d; id = hash3(c + vec3(7.7, 2.3, 5.9)); }
        else if (d < d2) { d2 = d; }
    }
    return vec3(sqrt(d1), sqrt(d2), id);
}

// Kirkwood's rock: andesitic volcanic breccia, the debris of old
// mudflows. Angular blocks of andesite in a grey-brown matrix, weathered
// into ribs and spires with vertical streaks; only faint banding where
// one flow sits on another. Built in 3D world space so walls don't
// stretch.
struct Rock {
    float h;      // relief in metres, for the bump lighting
    float id;     // boulder (about 2 m) hash
    float inside; // how far inside the boulder, 0 at the gap around it
    float streak; // vertical weathering streak, 0–1
    float grain;  // fine grain, 0–1, faded with distance
};

Rock rockField(vec3 p, float pxM) {
    Rock r;
    vec3 big  = worley3(p / 2.2 + vec3(0.0, fbm3(p / 11.0) * 0.6, 0.0));
    r.id      = big.z;
    r.inside  = smoothstep(0.0, 0.7, big.y - big.x);
    r.streak  = valueNoise3(p * vec3(0.9, 0.06, 0.9) + vec3(31.0, 0.0, 7.0));
    r.grain   = mix(0.5, fbm3(p / 0.35), 1.0 - smoothstep(0.1, 0.4, pxM));
    r.h       = r.inside * 0.2 - smoothstep(0.55, 0.8, r.streak) * 0.15 + (r.grain - 0.5) * 0.06;
    return r;
}

// Rock colour: a grey-brown matrix around boulders from dark andesite to
// pale grey with a few rusty ones, darker in the gaps and the streaks,
// fine grain, faint flow banding tens of metres apart, and broad
// oxidised patches.
vec3 rockAlbedo(vec3 p, Rock r) {
    vec3 matrixCol = vec3(0.45, 0.42, 0.39);
    vec3 dark      = vec3(0.33, 0.32, 0.32);
    vec3 pale      = vec3(0.56, 0.54, 0.51);
    vec3 rust      = vec3(0.55, 0.43, 0.34);
    vec3 boulder   = mix(dark, pale, r.id);
    boulder        = mix(boulder, rust, smoothstep(0.85, 0.97, fract(r.id * 7.31)));
    vec3 c         = mix(matrixCol, boulder, 0.7 * r.inside);
    c             *= 0.94 + 0.06 * r.inside;
    c             *= 0.86 + 0.28 * r.grain;
    c             *= 1.0 - 0.2 * smoothstep(0.5, 0.85, r.streak);
    float band     = valueNoise3(vec3(p.x / 60.0, (p.y + fbm3(p / 30.0) * 12.0) / 14.0, p.z / 60.0));
    c             *= 0.93 + 0.14 * band;
    float ox       = fbm3(p / 18.0 + vec3(5.0, 9.0, 1.0));
    c              = mix(c, c * vec3(1.12, 0.98, 0.86), smoothstep(0.55, 0.8, ox) * 0.6);
    return c;
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

// Track fade per game minute, as ln: world.TrackFadePerMinute = 0.985².
const float kTrackFade = 0.030227;

// trackTexel is one surface-detail texel's track intensity now: its R
// faded by the minutes since its clock (B low byte, A high byte).
float trackTexel(ivec2 p) {
    vec4 s = surfaceTexel(p);
    if (s.r <= 0.0) return 0.0;
    float then = floor(s.b * 255.0 + 0.5) + floor(s.a * 255.0 + 0.5) * 256.0;
    float age  = mod(uTrackClock - then, 65536.0);
    return s.r * exp(-kTrackFade * age);
}

// trackAt is the faded track intensity at uv, blended bilinearly by hand:
// each texel fades by its own clock, which filtering the packed clock
// bytes can't do.
float trackAt(vec2 uv) {
    vec2  p  = uv * uSurfaceSize - 0.5;
    ivec2 i0 = ivec2(floor(p));
    vec2  f  = p - floor(p);
    float a = trackTexel(i0),               b = trackTexel(i0 + ivec2(1, 0));
    float c = trackTexel(i0 + ivec2(0, 1)), d = trackTexel(i0 + ivec2(1, 1));
    return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
}

void main() {
#ifdef TUNNEL_MOUTHS
    if (inMouth(vWorldPos)) discard;
#endif
    vec4  snowState = vSnow;
    float snowDepth = vSnowDepth;
    if (uFragDetail > 0.5 && vInstabilityScore > -0.5) {
        cornerSnow(vWorldPos.xz, snowState, snowDepth);
    }
    if (uMaterialOn > 0.5 && vInstabilityScore > -0.5) {
        shoreSnow(vWorldPos.xz, snowState, snowDepth);
    }
    float grooming = clamp(snowState.x, 0.0, 1.0);
    float packed   = clamp(snowState.y, 0.0, 1.0);
    float ice      = clamp(snowState.z, 0.0, 1.0);
    // Metres of surface per pixel; fine detail fades out once it's sub-pixel.
    float pxM = max(length(dFdx(vWorldPos)), length(dFdy(vWorldPos)));
    // How big the moguls are: the 1 m mogul map up close, the cell
    // average once a metre is under a couple of pixels (the map has no
    // mips, so it would shimmer).
    float mogulFar = smoothstep(1.5, 3.0, pxM);
    float mogul    = mix(mogulSize(vWorldPos.xz), clamp(snowState.w, 0.0, 1.0), mogulFar);
    // Avalanche debris (world.Cell's Debris), shown where it's a couple
    // of centimetres deep and in full from 25 cm. Drives colour and
    // surface-roughness overrides below; suppresses powder and sparkle.
    // The edge is ragged, as debris tongues and lumps are.
    float isDebris = smoothstep(0.02, 0.25, debrisAt(vWorldPos.xz) * (0.5 + valueNoise(vWorldPos.xz / 4.0 + vec2(7.3, 1.9))));

    // Surface-detail sample — sub-cell features rendered from the
    // texture written by the simulation. R = skier tracks (faded by their
    // clock in B and A, trackAt), G = tree-well depth.
    vec4 surf = surfaceSample(vWorldPos.xz / uWorldSize);
    float trackR = trackAt(vWorldPos.xz / uWorldSize);
    // Normalise raw R (additive splats, 0.25 per pass) so one pass reads
    // as half-intensity and two saturate. This caps the crossing peak at
    // the same visual level as a single pass, fixing the blobby gradient
    // artifact where ∇R → 0 at a local maximum. The cat wipes tracks as
    // it grooms, so any on corduroy are fresh wear and show in full.
    float track = smoothstep(0.0, 0.5, trackR);
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
    float effDepth = max(snowDepth - 0.5 * well, 0.0);

    // Smoothed per-corner normal across the whole surface. Snow reads as
    // continuous; cliffs (handled by the existing slope-based rocky tint
    // on the snow palette below) still look distinct via colour. The
    // vNormal (per-triangle flat) attribute is no longer read by the FS —
    // the mesh geometry already carries the lane-edge step via the
    // corner-Y averaging, and the smoothed normal lets the natural slope
    // do the lighting rather than chopping it into per-triangle facets.
    vec3 N = normalize(vSmoothNormal);
    if (uFragDetail > 0.5 && uDetailOn > 0.5 && vInstabilityScore > -0.5) {
        vec2  texel = 1.0 / uDetailSize;
        vec2  uv    = (vWorldPos.xz / 1.25 + 0.5) * texel;
        float gx = (texture(uDetail, uv + vec2(texel.x, 0.0)).r - texture(uDetail, uv - vec2(texel.x, 0.0)).r) / 2.5;
        float gz = (texture(uDetail, uv + vec2(0.0, texel.y)).r - texture(uDetail, uv - vec2(0.0, texel.y)).r) / 2.5;
        N = normalize(vec3(N.x / N.y - gx, 1.0, N.z / N.y - gz));
    }
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
    // Rock: from the material map where there is one, else from slope.
    // rockW is how much of this point is rock; the rock itself is the
    // same procedural rock either way.
    float rockW     = 1.0 - smoothstep(0.55, 0.78, slope);
    vec3  notRock   = groundFlat;
    if (uMaterialOn > 0.5) {
        vec4 gmat = groundFromMaterial(vWorldPos, groundFlat);
        notRock   = gmat.rgb;
        rockW     = gmat.a;
    }
    Rock  rk;
    rk.h = 0.0;
    vec3  rockCol   = rock;
    // Skipped under 5 cm of snow or more, which hides the ground, except
    // where the material map says rock, which snow doesn't cover.
    if (rockW > 0.01 && (effDepth < 0.05 || uMaterialOn > 0.5)) {
        rk      = rockField(vWorldPos, pxM);
        rockCol = rockAlbedo(vWorldPos, rk);
    }
    vec3  ground    = mix(notRock, rockCol, rockW);

    // Snow albedo. Packed reads slightly bluer / darker than fresh powder;
    // modulate before mixing with ground so packed never leaks onto bare
    // rock. Shade colour comes from the sky fill, not from elevation.
    vec3 snow      = vec3(0.95, 0.96, 0.98);
    // Broad patchiness so a snowfield isn't one flat sheet: sheltered
    // patches a touch brighter and warmer, scoured ones greyer and cooler
    // (worked out a vertex: terrain.tese).
    float patchN = vPatch;
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
    // Rock in the material map holds a dusting at most (the sim sheds the
    // rest; see world.ShedSnow), lying in patches on its rough surface.
    // The cell's snow sits on the ground and ledges around it.
    if (uMaterialOn > 0.5 && rockW > 0.0) {
        float dust = smoothstep(0.55, 0.8, fbmNoise(vWorldPos.xz / 1.3 + vec2(vWorldPos.y * 0.25, 0.0)))
                   * smoothstep(0.02, 0.3, effDepth) * 0.85;
        snowness = mix(snowness, dust, rockW);
    }
    // Lakes (world.Lake, stepped by sim.StepLake): frozen ones are flat
    // snow, scoured in broad patches down to grey-blue ice where it's
    // thin; thin ice is dark with only patches of snow; open water takes
    // no snow at all.
    // Each look is weighted by how much of the four material samples
    // round the point it covers, roughened a little, so the ice's edge
    // follows a smooth line rather than the 1.25 m lattice (as the open
    // water's does below).
    if (uMaterialOn > 0.5) {
        vec2  g = vWorldPos.xz / 1.25;
        ivec2 p = ivec2(floor(g));
        vec2  f = fract(g);
        vec2  iw = vec2(0.0); // (ice, thin ice)
        for (int k = 0; k < 4; k++) {
            ivec2 o = ivec2(k & 1, k >> 1);
            float w = (o.x == 1 ? f.x : 1.0 - f.x) * (o.y == 1 ? f.y : 1.0 - f.y);
            int   m = materialAt(p + o);
            iw += w * vec2(m == MAT_ICE ? 1.0 : 0.0, m == MAT_THIN_ICE ? 1.0 : 0.0);
        }
        // The noise only near ice (the roughening is at most ±0.125, so
        // under 0.2 of ice round the point there's none to show).
        if (max(iw.x, iw.y) > 0.2) {
            float rough = 0.25 * (valueNoise(vWorldPos.xz / 0.9) - 0.5);
            iw = smoothstep(vec2(0.35), vec2(0.65), iw + rough);
            float scour = smoothstep(0.45, 0.7, fbmNoise(vWorldPos.xz / 30.0 + vec2(13.1, 4.7)));
            float thin  = 1.0 - smoothstep(0.1, 0.4, effDepth);
            snowness = mix(snowness, min(snowness, 1.0 - 0.9 * scour * thin), iw.x);
            snowness = mix(snowness, min(snowness, 0.6 * smoothstep(0.55, 0.75, fbmNoise(vWorldPos.xz / 12.0 + vec2(3.3, 9.1)))), iw.y);
        }
    }
    // Open water (lakes and creeks) blends across the four material
    // samples around the point, with a little noise, so its edge follows
    // a smooth, slightly ragged line rather than the 1.25 m lattice.
    // Averaged over a few metres as the snow's depth is (terrain.tese
    // openWaterAt), so the snow's edge along a creek slanting across the
    // lattice is a smooth bank, not a staircase.
    float openW = 0.0;
    if (uMaterialOn > 0.5) {
        vec2  g   = vWorldPos.xz / 1.25;
        float sum = 0.0;
        for (int j = -1; j <= 1; j++) {
            for (int i = -1; i <= 1; i++) {
                vec2  q = g + vec2(i, j);
                ivec2 p = ivec2(floor(q));
                vec2  f = fract(q);
                float o00 = materialAt(p) == MAT_OPEN_WATER ? 1.0 : 0.0;
                float o10 = materialAt(p + ivec2(1, 0)) == MAT_OPEN_WATER ? 1.0 : 0.0;
                float o01 = materialAt(p + ivec2(0, 1)) == MAT_OPEN_WATER ? 1.0 : 0.0;
                float o11 = materialAt(p + ivec2(1, 1)) == MAT_OPEN_WATER ? 1.0 : 0.0;
                sum += mix(mix(o00, o10, f.x), mix(o01, o11, f.x), f.y);
            }
        }
        openW = smoothstep(0.15, 0.55, sum / 9.0 + 0.12 * (valueNoise(vWorldPos.xz / 0.9) - 0.5));
    }
    snowness *= 1.0 - openW;

    // Avalanche debris: white blocks of tumbled snow, big ones a couple
    // of metres across with smaller chunks between, each its own shade,
    // with cool shadowed cracks and the odd fleck of dirt and rock picked
    // up on the way down. Once the blocks are too small on screen, a
    // coarse mottle instead.
    vec4  dBig = vec4(1.0), dSmall = vec4(1.0);
    float dNear = 0.0;
    if (isDebris > 0.0 && snowness > 0.05) {
        dNear = 1.0 - smoothstep(0.35, 0.9, pxM);
        float hollow = 0.0, fleck = 0.0, tone = 1.0;
        if (dNear > 0.0) {
            // Warped so the lumps are irregular, not a tiling of cells.
            vec2 warp = vec2(valueNoise(vWorldPos.xz / 1.7), valueNoise(vWorldPos.xz / 1.7 + vec2(5.2, 1.3))) - 0.5;
            vec2 pb = vWorldPos.xz / 2.0 + warp * 1.2, ps = vWorldPos.xz / 0.8 + vec2(31.7, 12.3) + warp * 1.6;
            dBig   = debrisBlocks(pb);
            dSmall = debrisBlocks(ps);
            // Shadowed hollows where lumps meet, away from their tops.
            hollow = (0.7 * smoothstep(0.35, 0.8, dBig.x) + 0.3 * smoothstep(0.35, 0.8, dSmall.x)) * dNear;
            tone   = mix(1.0, 0.95 + 0.07 * hash3(vec3(floor(pb - dBig.zw), 5.0)), dNear);
            fleck  = step(0.997, hash3(vec3(floor(vWorldPos.xz / 0.3), 11.0))) * dNear;
        }
        float mottle = fbmNoise(vWorldPos.xz / 3.0 + vec2(5.1, 2.7));
        vec3  shade  = mix(vec3(1.0), vec3(0.84, 0.88, 0.96), hollow) * tone;
        shade *= mix(1.0, 0.9 + 0.14 * mottle, 1.0 - 0.5 * dNear);
        vec3  dc = mix(snow * shade, vec3(0.42, 0.38, 0.34), fleck * 0.7);
        snow = mix(snow, dc, isDebris);
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

    // Screen-space derivatives for the rock bump, taken here in uniform
    // control flow; rk.h is 0 where no rock was computed.
    vec3  dPdx = dFdx(vWorldPos), dPdy = dFdy(vWorldPos);
    float dHdx = dFdx(rk.h),      dHdy = dFdy(rk.h);
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
            vec2 g = vDrift; // worked out a vertex (terrain.tese)
            const float driftAmp = 0.6; // metres
            kick.xz -= g * driftAmp * driftness;
        }
        // Moguls: lit from the mogul field's slope (mogul.glsl), the same
        // surface the tessellation raises up close. Gone once a bump spans
        // too few pixels; the far shading below stands in.
        float mogulK = (1.0 - smoothstep(1.0, 2.5, pxM)) * snowness;
        if (mogul > 0.01 && mogulK > 0.01) {
            const float e = 0.25;
            vec2  p0 = vWorldPos.xz;
            vec2  d  = mogulFallLine(p0);
            float h0 = mogulHeight(p0, d);
            vec2  g  = vec2(mogulHeight(p0 + vec2(e, 0.0), d) - h0,
                            mogulHeight(p0 + vec2(0.0, e), d) - h0) / e;
            kick.xz -= g * mogulK;
        }
        if (well > 0.01) {
            // ∇G via offset samples — the well texture is in metres of
            // depth-loss at the trunk, so each unit of well gives ~0.5 m
            // of displacement. Sample offsets are scaled into UV space.
            vec2 uvEps = vec2(bumpEps, 0) / uWorldSize;
            vec2 uvEpz = vec2(0, bumpEps) / uWorldSize;
            vec2 uv = vWorldPos.xz / uWorldSize;
            float g0 = well;
            float gx = surfaceSample(uv + uvEps).g;
            float gz = surfaceSample(uv + uvEpz).g;
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
            float rx = smoothstep(0.0, 0.5, trackAt(uv + uvEps));
            float rz = smoothstep(0.0, 0.5, trackAt(uv + uvEpz));
            const float trackAmp = 0.08; // metres — shallower than wells
            kick.x += (rx - r0) / bumpEps * trackAmp;
            kick.z += (rz - r0) / bumpEps * trackAmp;
        }
        // Debris blocks, each lit as a dome from its centre, over a coarse
        // lumpy relief that carries on once they're too small to see.
        // Fires instead of the powder kick (powderness is zeroed on debris).
        if (isDebris > 0.0 && snowness > 0.1) {
            if (dNear > 0.0) {
                kick.xz += (dBig.zw * 0.7 + dSmall.zw * 0.4) * isDebris * dNear;
            }
            vec2 g = fbmGrad(vWorldPos.xz / 3.0 + vec2(53.1, 17.8), pxM / 3.0) / 3.0;
            const float debrisAmp = 0.6; // metres
            kick.xz -= g * debrisAmp * isDebris;
        }
        // Corduroy: ridges run along the cat's heading, about 25 cm apart
        // (exaggerated so they read from the game camera), and are lit as
        // bumps so they catch a low sun. Fresh skier tracks flatten them.
        if (cordAmp > 0.01) {
            kick.xz -= cordAcross * cos(cordPhase) * 0.35 * cordAmp;
        }
        // Rock relief, from the screen-space change in rockField's
        // height (surface-gradient bump mapping), so it costs nothing
        // extra and works on walls at any angle.
        float rockVis = rockW * (1.0 - snowness) * (1.0 - smoothstep(1.0, 2.0, pxM));
        if (rockVis > 0.02) {
            vec3  r1  = cross(dPdy, N);
            vec3  r2  = cross(N, dPdx);
            float det = dot(dPdx, r1);
            if (abs(det) > 1e-8) {
                vec3 surfGrad = sign(det) * (dHdx * r1 + dHdy * r2) / abs(det);
                kick -= surfGrad * rockVis;
            }
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
    // A mogul field too far off to light bump by bump reads a little
    // darker, from the shadows in its troughs.
    shaded *= 1.0 - 0.12 * mogul * smoothstep(1.0, 2.5, pxM) * snowness;

    // Baked AO shades only the sky fill (valleys and cliff bases see less
    // sky); direct sun is already handled by sunVis.
    vec3 lit = shaded * (fillLight(Nshading) * vAO + 0.85 * diff * uSunColor);
    if (uLampCount > 0) {
        lit += base * lampLight(vWorldPos, Nshading) * vAO;
    }
    lit += base * nightLight(vWorldPos) * vAO;
    // Open water mirrors the sky, more at grazing angles, and glints
    // where small ripples catch the sun.
    if (openW > 0.01) {
        vec3  V    = normalize(uCameraPos - vWorldPos);
        vec2  rip  = valueNoiseD(vWorldPos.xz / 1.5 + vec2(uTime * 0.15, uTime * 0.1)).yz / 1.5;
        vec3  Nw   = normalize(N + vec3(rip.x, 0.0, rip.y) * 0.15);
        float fres = pow(1.0 - max(dot(Nw, V), 0.0), 4.0);
        vec3 water = mix(lit, fillLight(Nw) * 1.1, 0.2 + 0.6 * fres);
        water += uSunColor * pow(max(dot(Nw, normalize(L + V)), 0.0), 150.0) * sunVis * 2.0;
        lit = mix(lit, water, openW);
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
        // Only a cell in five (more on ice) is a facet at all, so the rest
        // stop at that.
        vec3 cell = floor(vWorldPos * 4.0);
        if (sparkleK > 0.0 && sunVis > 0.0 && hash3(cell + 73.0) >= 0.80 - ice * 0.15) {
            vec3  tilt = vec3(hash3(cell), hash3(cell + 17.0), hash3(cell + 41.0)) * 2.0 - 1.0;
            vec3  facet = normalize(Nshading + tilt * 0.6);
            float glint = pow(max(dot(facet, H), 0.0), 600.0);
            float spec = glint * dot(uSunColor, vec3(1.0 / 3.0)) * sunVis * sparkleK;
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
        float d = clamp(snowDepth / 5.0, 0.0, 1.0);
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

    // Avalanche danger: slide paths by their danger level
    // (world.Terrain.AvyDangerShade): 0 off any path, then Low 0.2,
    // Moderate 0.4, Danger 0.6, High 0.8, Extreme 1.0, as the danger
    // scale's colours: green, yellow, orange, red, black.
    if ((uOverlayMode & 16) != 0 && vInstabilityScore > 0.05) {
        float lv = clamp(vInstabilityScore * 5.0 - 1.0, 0.0, 4.0);
        vec3 c0 = vec3(0.20, 0.70, 0.25);
        vec3 c1 = vec3(0.95, 0.85, 0.15);
        vec3 c2 = vec3(0.97, 0.55, 0.10);
        vec3 c3 = vec3(0.88, 0.15, 0.10);
        vec3 c4 = vec3(0.10, 0.08, 0.08);
        vec3 riskCol = lv < 1.0 ? mix(c0, c1, lv) : lv < 2.0 ? mix(c1, c2, lv - 1.0)
                     : lv < 3.0 ? mix(c2, c3, lv - 2.0) : mix(c3, c4, lv - 3.0);
        fragColor.rgb = mix(fragColor.rgb, riskCol, 0.65);
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
    // CPU→GPU pipeline is visible from the testbed. R=tracks (faded),
    // G=tree wells. Replaces base shading like the bump-normal
    // overlay so the channels are legible on their own. Bound to `N`.
    if ((uOverlayMode & 256) != 0) {
        fragColor.rgb = vec3(trackR, surf.g, 0.0);
    }

    // Ground overlay — what the ground is made of, whatever covers it:
    // meadow green, dirt brown, scree pale grey, rock near-black; lakes
    // pale blue when frozen, mid blue on thin ice, deep blue when open.
    if ((uOverlayMode & 2048) != 0 && uMaterialOn > 0.5 && vInstabilityScore > -0.5) {
        int  m = materialAt(ivec2(floor(vWorldPos.xz / 1.25 + 0.5)));
        vec3 col = vec3(0.35, 0.70, 0.30);
        if (m == MAT_DIRT)      col = vec3(0.60, 0.42, 0.25);
        if (m == MAT_SCREE)     col = vec3(0.78, 0.78, 0.74);
        if (m == MAT_ROCK)      col = vec3(0.14, 0.14, 0.18);
        if (m == MAT_ICE)        col = vec3(0.55, 0.80, 1.00);
        if (m == MAT_THIN_ICE)   col = vec3(0.30, 0.55, 0.95);
        if (m == MAT_OPEN_WATER) col = vec3(0.05, 0.20, 0.80);
        fragColor.rgb = mix(fragColor.rgb, col, 0.8);
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
