#version 410 core

// Open water: dark and clear, darker over deep water, the sky in it at a
// glancing angle (Fresnel), the sun glinting off slow ripples.

in vec3  vWorldPos;
in float vAlpha;
in float vDepth;

uniform vec3  uViewDir; // unit, from the camera into the scene
uniform float uTime;

out vec4 fragColor;

float wHash(vec2 p) {
    p = fract(p * vec2(443.897, 441.423));
    p += dot(p, p.yx + 19.19);
    return fract((p.x + p.y) * p.x);
}

float wNoise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    vec2 u = f * f * (3.0 - 2.0 * f);
    return mix(mix(wHash(i), wHash(i + vec2(1, 0)), u.x),
               mix(wHash(i + vec2(0, 1)), wHash(i + vec2(1, 1)), u.x), u.y);
}

// ripples is the water's height pattern at xz, two drifting layers.
float ripples(vec2 xz) {
    return wNoise(xz / 1.7 + vec2(uTime * 0.11, uTime * 0.07)) * 0.6
         + wNoise(xz / 0.6 - vec2(uTime * 0.17, -uTime * 0.13)) * 0.4;
}

void main() {
    vec2  xz = vWorldPos.xz;
    const float e = 0.15;
    float h  = ripples(xz);
    vec3  n  = normalize(vec3((h - ripples(xz + vec2(e, 0.0))) * 0.6 / e, 1.0,
                              (h - ripples(xz + vec2(0.0, e))) * 0.6 / e));
    vec3  v  = -uViewDir;
    // The sky in the water: a little from above, much more at a glance.
    float fresnel = 0.18 + 0.82 * pow(1.0 - clamp(dot(n, v), 0.0, 1.0), 3.0);

    float shallow = exp(-vDepth * 0.6);
    vec3  deep    = vec3(0.05, 0.11, 0.14);
    vec3  near    = vec3(0.13, 0.24, 0.25);
    vec3  body    = mix(deep, near, shallow) * sceneLight();
    vec3  sky     = sceneLight() * vec3(0.62, 0.72, 0.86);
    vec3  col     = mix(body, sky, fresnel);

    vec3  hv   = normalize(uSunDir + v);
    float spec = pow(max(dot(n, hv), 0.0), 90.0) * 0.6;
    col += uSunColor * spec;

    float alpha = vAlpha * mix(0.93, 0.8, shallow);
    fragColor = vec4(toneMap(applyHaze(col, vWorldPos)), alpha);
}
