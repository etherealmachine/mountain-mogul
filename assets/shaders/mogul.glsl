// Shared by the terrain stages (prepended like lighting.glsl).

// The terrain extent in metres (cells × 5): world xz / uWorldSize is the
// UV of every per-metre map (surface detail, groom, moguls).
uniform vec2 uWorldSize;

// Mogul field, mirrored by world.MogulHeightAt so skiers ride the drawn
// bumps. uMogulMap is world.MogulMap (how big the moguls are, one texel per
// metre); uMogulDir is the smoothed fall line per cell, (dx, dz) mapped to
// 0..1. The bumps sit in a staggered lattice, mogulSU apart across the
// fall line and mogulSV down it, so the lanes between them run diagonally,
// where skiers turn. Each mogulTile-metre tile lays its lattice out from
// its own centre along the local fall line, so the field follows the slope
// as it bends; neighbouring tiles blend over their shared edge, and a
// gentle warp keeps the rows from looking ruled.
uniform sampler2D uMogulMap;
uniform sampler2D uMogulDir;

const float mogulAmp   = 0.45; // metres, crest above the mean at full moguls
const float mogulSU    = 6.0;
const float mogulSV    = 7.0;
const float mogulTile  = 12.0;
const float mogulWarp  = 0.9;
const float mogulWarpL = 9.0;

float mogulHash(ivec2 p, uint s) {
    uint h = (uint(p.x) * 0x8da6b343u) ^ (uint(p.y) * 0xd8163841u) ^ (s * 0xcb1ab31fu);
    h ^= h >> 15; h *= 0x2c1b3c6du;
    h ^= h >> 12; h *= 0x297a2d39u;
    h ^= h >> 15;
    return float(h >> 8) / 16777216.0;
}

float mogulNoise(vec2 p, uint s) {
    ivec2 i = ivec2(floor(p));
    vec2  f = fract(p);
    vec2  u = f * f * (3.0 - 2.0 * f);
    return mix(mix(mogulHash(i, s),              mogulHash(i + ivec2(1, 0), s), u.x),
               mix(mogulHash(i + ivec2(0, 1), s), mogulHash(i + ivec2(1, 1), s), u.x), u.y);
}

// mogulShape is the mogul surface at world xz, about -1..1, for fall line d.
float mogulShape(vec2 xz, vec2 d) {
    vec2 q = xz / mogulWarpL;
    xz += (vec2(mogulNoise(q, 3u), mogulNoise(q, 4u)) * 2.0 - 1.0) * mogulWarp;
    vec2 q2 = xz / (mogulWarpL / 3.0);
    xz += (vec2(mogulNoise(q2, 6u), mogulNoise(q2, 7u)) * 2.0 - 1.0) * (mogulWarp * 0.6);
    vec2  g = xz / mogulTile - 0.5;
    ivec2 i = ivec2(floor(g));
    vec2  f = fract(g);
    vec2  w = smoothstep(0.3, 0.7, f);
    float sum = 0.0, w2 = 0.0;
    for (int k = 0; k < 4; k++) {
        ivec2 o  = ivec2(k & 1, k >> 1);
        ivec2 ti = i + o;
        float wk = (o.x == 1 ? w.x : 1.0 - w.x) * (o.y == 1 ? w.y : 1.0 - w.y);
        if (wk <= 0.0) continue;
        vec2  r = xz - (vec2(ti) + 0.5) * mogulTile;
        float u = dot(r, vec2(d.y, -d.x)) / mogulSU + mogulHash(ti, 1u);
        float v = dot(r, d) / mogulSV + mogulHash(ti, 2u);
        sum += wk * cos(6.2831853 * u) * cos(6.2831853 * v);
        w2  += wk * wk;
    }
    // Mounds pushed up and troughs carved down as far, each fattened so
    // neighbours meet over saddles, and each its own size.
    float h = clamp(sum / sqrt(max(w2, 1e-6)), -1.0, 1.0);
    return h * (1.5 - 0.5 * h * h) * (0.65 + 0.35 * mogulNoise(xz / 4.0, 5u));
}

// mogulSize is how big the moguls are at world xz, 0..1.
float mogulSize(vec2 xz) {
    return textureLod(uMogulMap, xz / uWorldSize, 0.0).r;
}

// mogulFallLine is the smoothed fall line at world xz.
vec2 mogulFallLine(vec2 xz) {
    vec2 d = textureLod(uMogulDir, xz / uWorldSize, 0.0).rg * 2.0 - 1.0;
    float l = length(d);
    return l > 1e-3 ? d / l : vec2(0.0, 1.0);
}

// mogulHeight is the mogul surface's height above the snow at world xz.
float mogulHeight(vec2 xz, vec2 d) {
    float s = mogulSize(xz);
    return s > 0.002 ? mogulAmp * s * mogulShape(xz, d) : 0.0;
}
