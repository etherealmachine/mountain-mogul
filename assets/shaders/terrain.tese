#version 410 core
layout(triangles, equal_spacing, ccw) in;

uniform mat4 uViewProj;

// Sub-cell height detail (world.TerrainDetail): one texel per 1.25 m of
// the corner grid, added to the flat triangle surface.
uniform sampler2D uDetail;
uniform float     uDetailOn;
uniform vec2      uDetailSize;
// uFragDetail is 1 when the terrain is drawn at a coarse level of detail:
// the fragment shader tilts the normal by the detail per pixel instead.
uniform float     uFragDetail;

// The material map (see terrain.frag): snow doesn't build up on rock.
uniform sampler2D uMaterial;
uniform float     uMaterialOn;
uniform vec2      uMaterialSize;

// materialTexel is material sample p's world.Material.
float materialTexel(ivec2 p) {
    ivec2 hi = ivec2(uMaterialSize) - 1;
    return texelFetch(uMaterial, clamp(p, ivec2(0), hi), 0).r * 255.0;
}

// rockTexel is 1 where material sample p is rock (world.MatRock, 3),
// which holds no snow depth.
float rockTexel(ivec2 p) {
    return abs(materialTexel(p) - 3.0) < 0.5 ? 1.0 : 0.0;
}

// waterTexel is 1 where material sample p is open water
// (world.MatOpenWater, 6), which holds no snow either.
float waterTexel(ivec2 p) {
    return abs(materialTexel(p) - 6.0) < 0.5 ? 1.0 : 0.0;
}

// rockAt is how much of the ground around world xz is rock, blending the
// four material samples around it.
float rockAt(vec2 xz) {
    vec2  g = xz / 1.25;
    ivec2 p = ivec2(floor(g));
    vec2  f = fract(g);
    return mix(mix(rockTexel(p),               rockTexel(p + ivec2(1, 0)), f.x),
               mix(rockTexel(p + ivec2(0, 1)), rockTexel(p + ivec2(1, 1)), f.x), f.y);
}

float waterBilinear(vec2 g) {
    ivec2 p = ivec2(floor(g));
    vec2  f = fract(g);
    return mix(mix(waterTexel(p),               waterTexel(p + ivec2(1, 0)), f.x),
               mix(waterTexel(p + ivec2(0, 1)), waterTexel(p + ivec2(1, 1)), f.x), f.y);
}

// openWaterAt is how much the snow round world xz gives way to open
// water: the water samples averaged over a few metres, eased. Taken
// sample by sample, a creek running at a slant to the 1.25 m lattice
// has a staircase edge, and a deep snowpack cut off along it stands in a
// sawtooth wall; averaged, the snow slopes down into the water as a
// bank, and over the water itself (the average high) it's gone.
float openWaterAt(vec2 xz) {
    vec2  g = xz / 1.25;
    float sum = 0.0;
    for (int j = -1; j <= 1; j++) {
        for (int i = -1; i <= 1; i++) {
            sum += waterBilinear(g + vec2(i, j));
        }
    }
    return smoothstep(0.15, 0.55, sum / 9.0);
}

// Per-corner snow state (terrain.vert): blended between the four
// corners round a point by its grid position, not across the coarse
// triangle, so the edge of the snow along a narrow sharp feature (a
// creek's bank) is smooth instead of following the triangles' diagonals
// in a sawtooth. VisualElevationAt blends the same.
uniform sampler2D uCornerSnowA;
uniform sampler2D uCornerSnowB;

void cornerSnowAt(vec2 g, out vec4 a, out float depth) {
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

patch in vec3  tcNormal;
      in vec2  tcGrid[];
      in vec2  tcKind[];
      in float tcSmoothY[];
      in float tcAO[];
      in vec4  tcSnow[];
      in float tcSnowDepth[];
      in vec3  tcSmoothNormal[];
      in float tcInstabilityScore[];

flat out vec3  vNormal;
     out vec3  vWorldPos;
     out float vSmoothY;
     out float vAO;
     out vec4  vSnow;
     out float vSnowDepth;
     out vec3  vSmoothNormal;
     out float vInstabilityScore;
// vPatch is the snow's broad patchiness (terrain.frag), tens of metres
// across, so worked out here a vertex rather than a pixel.
     out float vPatch;
// vDrift is the wind drifts' slope (terrain.frag's normal kick), tens of
// metres across, likewise.
     out vec2  vDrift;

// Noise functions mirrored from terrain.frag — same hash/fbm so TES
// displacement matches the fragment-shader normal-map exactly.
float hash3(vec3 p) {
    p = fract(p * vec3(443.897, 441.423, 437.195));
    p += dot(p, p.yzx + 19.19);
    return fract((p.x + p.y) * p.z);
}

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

// valueNoiseD is valueNoise with its gradient (terrain.frag's).
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

float fbmNoise(vec2 p) {
    float n  = valueNoise(p);
          n += 0.5  * valueNoise(p * 2.0);
          n += 0.25 * valueNoise(p * 4.0);
    return n / 1.75;
}

#define BARY(a) (gl_TessCoord.x*(a)[0] + gl_TessCoord.y*(a)[1] + gl_TessCoord.z*(a)[2])

void main() {
    // Barycentric position from patch control points (world space, pre-projection).
    vec3 pos = gl_TessCoord.x * gl_in[0].gl_Position.xyz
             + gl_TessCoord.y * gl_in[1].gl_Position.xyz
             + gl_TessCoord.z * gl_in[2].gl_Position.xyz;

    float snowDepth = BARY(tcSnowDepth);
    vec4  snow      = BARY(tcSnow);
    float smoothY   = BARY(tcSmoothY);
    float ao        = BARY(tcAO);
    vec3  smoothN   = normalize(BARY(tcSmoothNormal));
    float instab    = BARY(tcInstabilityScore);
    vec3  faceN     = tcNormal;

    vec2 kind = BARY(tcKind);
    if (kind.y < 0.5) {
        cornerSnowAt(BARY(tcGrid), snow, snowDepth);
    }
    if (uDetailOn > 0.5 && kind.x > 0.0) {
        vec2  texel = 1.0 / uDetailSize;
        vec2  uv    = (BARY(tcGrid) * 4.0 + 0.5) * texel;
        pos.y += texture(uDetail, uv).r * kind.x;
        if (kind.y < 0.5 && uFragDetail < 0.5) {
            // Tilt both normals by the detail's slope (1.25 m per texel).
            float gx = (texture(uDetail, uv + vec2(texel.x, 0.0)).r - texture(uDetail, uv - vec2(texel.x, 0.0)).r) / 2.5;
            float gz = (texture(uDetail, uv + vec2(0.0, texel.y)).r - texture(uDetail, uv - vec2(0.0, texel.y)).r) / 2.5;
            faceN   = normalize(vec3(faceN.x / faceN.y - gx, 1.0, faceN.z / faceN.y - gz));
            smoothN = normalize(vec3(smoothN.x / smoothN.y - gx, 1.0, smoothN.z / smoothN.y - gz));
        }
    }

    float packed     = clamp(snow.y, 0.0, 1.0);
    float powderness = (1.0 - packed) * smoothstep(0.0, 0.5, snowDepth);

    // Base snow height — separates density differences geometrically.
    // snowDepth = accumulation / density, so powder (low density) sits
    // higher than the same SWE in groomed or packed form. The cell's snow
    // lies on its ground and ledges, not its rock, so rock faces and the
    // lips above them stay sharp instead of blanketed.
    if (uMaterialOn > 0.5) {
        snowDepth *= (1.0 - rockAt(pos.xz)) * (1.0 - openWaterAt(pos.xz));
    }
    pos.y += snowDepth;

    // Powder noise — same amplitude as the procedural normal-map kick
    // in terrain.frag, converting the fake bump into actual geometry. The FS normal-map kick remains as a detail layer on top.
    float disp = 0.0;
    if (powderness > 0.05) {
        vec2 off = vec2(17.3, 91.7);
        disp += fbmNoise(pos.xz / 5.0 + off) * 0.10 * powderness;
    }
    // The mogul field (mogul.glsl); world.MogulHeightAt adds the same, so
    // skiers ride the bumps.
    disp += mogulHeight(pos.xz, mogulFallLine(pos.xz));
    pos.y += disp;

    vNormal           = faceN;
    vWorldPos         = pos;
    vSmoothY          = smoothY;
    vAO               = ao;
    vSnow             = snow;
    vSnowDepth        = snowDepth;
    vSmoothNormal     = smoothN;
    vInstabilityScore = instab;
    vDrift            = valueNoiseD(pos.xz / 22.0 + vec2(311.7, 47.3)).yz / 22.0;
    vPatch            = valueNoise(pos.xz / 40.0 + vec2(71.3, 12.9)) * 0.65
                      + valueNoise(pos.xz / 9.0 + vec2(5.1, 33.7)) * 0.35;

    gl_Position = uViewProj * vec4(pos, 1.0);
}
