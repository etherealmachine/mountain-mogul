#version 410 core

in vec3 vColor;
flat in vec3 vNormal;
in vec3 vSmoothNormal;
in vec3 vWorldPos;

uniform vec2  uBrushCenter;
uniform float uBrushRadius;
uniform int   uOverlayMode; // 0 = normal, 1 = slope + contour

out vec4 fragColor;

// lighting.glsl is prepended by shader loader

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

void main() {
    vec3 nSmooth = normalize(vSmoothNormal);
    vec3 triW = triWeights(nSmooth);

    // Flat faces → snow, steep/vertical faces → rock
    vec3 snowColor = vec3(0.90, 0.94, 1.00);
    vec3 rockColor = vec3(0.52, 0.49, 0.46);
    vec3 baseColor = mix(rockColor, snowColor, triW.y);

    // Subtle noise to break up flat uniformity
    float detail = triDetail(vWorldPos, 0.018) * 0.6
                 + triDetail(vWorldPos, 0.45)  * 0.4;
    baseColor *= mix(0.88, 1.08, detail);

    vec3 lit = computeLighting(nSmooth, baseColor);
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

        // Contour lines every 50 m; fwidth keeps line width constant on screen
        float elevMod = mod(vWorldPos.y, 50.0);
        float fw      = fwidth(vWorldPos.y);
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
