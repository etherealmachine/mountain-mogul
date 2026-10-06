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

// rockTexel is 1 where material sample p holds no snow depth: rock
// (world.MatRock, 3) and open water (world.MatOpenWater, 6).
float rockTexel(ivec2 p) {
    ivec2 hi = ivec2(uMaterialSize) - 1;
    float m  = texelFetch(uMaterial, clamp(p, ivec2(0), hi), 0).r * 255.0;
    return (abs(m - 3.0) < 0.5 || abs(m - 6.0) < 0.5) ? 1.0 : 0.0;
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
    float mogul      = clamp(snow.w, 0.0, 1.0);
    float powderness = (1.0 - packed) * smoothstep(0.0, 0.5, snowDepth);

    // Base snow height — separates density differences geometrically.
    // snowDepth = accumulation / density, so powder (low density) sits
    // higher than the same SWE in groomed or packed form. The cell's snow
    // lies on its ground and ledges, not its rock, so rock faces and the
    // lips above them stay sharp instead of blanketed.
    if (uMaterialOn > 0.5) {
        snowDepth *= 1.0 - rockAt(pos.xz);
    }
    pos.y += snowDepth;

    // Noise kicks — same amplitudes as the procedural normal-map
    // kicks in terrain.frag (lines 212-231), converting the fake bump into
    // actual geometry. The FS normal-map kick remains as a detail layer on top.
    float disp = 0.0;
    if (powderness > 0.05) {
        vec2 off = vec2(17.3, 91.7);
        disp += fbmNoise(pos.xz / 5.0 + off) * 0.10 * powderness;
    }
    if (mogul > 0.01) {
        float h = fbmNoise(pos.xz / 3.0) * 0.6 + fbmNoise(pos.xz * 2.1 / 3.0) * 0.4;
        disp += h * 0.8 * mogul;
    }
    pos.y += disp;

    vNormal           = faceN;
    vWorldPos         = pos;
    vSmoothY          = smoothY;
    vAO               = ao;
    vSnow             = snow;
    vSnowDepth        = snowDepth;
    vSmoothNormal     = smoothN;
    vInstabilityScore = instab;

    gl_Position = uViewProj * vec4(pos, 1.0);
}
