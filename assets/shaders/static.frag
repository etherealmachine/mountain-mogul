#version 410 core

in vec3 vColor;
flat in vec3 vNormal;
in vec2 vTexCoord;
flat in float vPerceived;
in vec3 vWorldPos;
in float vLocalY;
flat in float vInstanceHash;

uniform sampler2D uTexture;
uniform float uAlpha;

// Foliage mode for the conifer batches: wrap lighting through the canopy,
// per-tree colour variation, darker low branches, and snow on the boughs.
uniform int   uFoliage;
uniform float uFoliageHeight; // model top, model units
uniform float uCanopySnow;    // 0..1 snow load on branches

float foliageHash(vec3 p) {
    p = fract(p * vec3(443.897, 441.423, 437.195));
    p += dot(p, p.yzx + 19.19);
    return fract((p.x + p.y) * p.z);
}

// Smooth 3D value noise for soft snow patches on the canopy.
float foliageNoise(vec3 p) {
    vec3 i = floor(p);
    vec3 f = fract(p);
    vec3 u = f * f * (3.0 - 2.0 * f);
    float a = mix(foliageHash(i),                   foliageHash(i + vec3(1, 0, 0)), u.x);
    float b = mix(foliageHash(i + vec3(0, 1, 0)),   foliageHash(i + vec3(1, 1, 0)), u.x);
    float c = mix(foliageHash(i + vec3(0, 0, 1)),   foliageHash(i + vec3(1, 0, 1)), u.x);
    float d = mix(foliageHash(i + vec3(0, 1, 1)),   foliageHash(i + vec3(1, 1, 1)), u.x);
    return mix(mix(a, b, u.y), mix(c, d, u.y), u.z);
}

vec3 shadeFoliage(vec3 baseColor) {
    vec3  N   = normalize(vNormal);
    float hgt = clamp(vLocalY / max(uFoliageHeight, 0.1), 0.0, 1.0);
    vec3  c   = baseColor;
    // Per-tree variation: brightness, and a shift toward yellow-green.
    c *= mix(0.80, 1.15, vInstanceHash);
    c  = mix(c, c * vec3(1.10, 1.05, 0.80), fract(vInstanceHash * 7.13) * 0.6);
    // Low branches sit in their own shade; tips catch the light.
    c *= mix(0.65, 1.15, hgt);
    // Snow on the boughs: soft bands where tiers of branches hold it,
    // broken up by noise, heavier toward the top, never on the trunk.
    float n     = foliageNoise(vWorldPos * 1.4);
    float bough = 0.5 + 0.5 * sin(vLocalY * 4.0 + n * 4.0 + vInstanceHash * 6.28);
    float load  = smoothstep(0.75, 0.9, bough * 0.55 + n * 0.45 + hgt * 0.25 - (1.0 - uCanopySnow) * 0.6);
    load *= uCanopySnow * smoothstep(0.15, 0.3, hgt);
    c = mix(c, vec3(0.93, 0.95, 0.98), load * 0.9);
    // Wrap lighting: light passes through the needles, so the terminator is soft.
    float ndl  = dot(N, uSunDir);
    float diff = clamp((ndl + 0.45) / 1.45, 0.0, 1.0) * keyLightVisibility(vWorldPos, N);
    return c * (fillLight(N) + diff * uSunColor);
}

out vec4 fragColor;

// lighting.glsl is prepended by shader loader

void main() {
    vec3 texColor = texture(uTexture, vTexCoord).rgb;
    vec3 baseColor = texColor * vColor;
    vec3 lit = uFoliage == 1 ? shadeFoliage(baseColor)
                             : computeLightingAt(vNormal, baseColor, vWorldPos);
    if (uLampCount > 0) {
        lit += baseColor * lampLight(vWorldPos, normalize(vNormal));
    }

    // Followed-skier perception highlight: warm yellow at ~50% mix.
    if (vPerceived > 0.5) {
        lit = mix(lit, vec3(1.0, 0.95, 0.1), 0.5);
    }

    fragColor = vec4(toneMap(applyHaze(lit, vWorldPos)), uAlpha);
}
