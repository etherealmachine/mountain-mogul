#version 410 core

flat in vec3  vNormal;
in vec3  vWorldPos;
in float vSmoothY;
in float vAO;

uniform vec2  uBrushCenter;
uniform float uBrushRadius;
uniform int   uOverlayMode;     // 0 = off, 1 = contour, 2 = slope debug
uniform vec3  uCameraPos;
uniform float uTime;
uniform float uTerrainMinY;
uniform float uTerrainMaxY;

out vec4 fragColor;

// Cell-hash for the sparkle pass — cheap integer mixing in float space.
float hash3(vec3 p) {
    p = fract(p * vec3(443.897, 441.423, 437.195));
    p += dot(p, p.yzx + 19.19);
    return fract((p.x + p.y) * p.z);
}

void main() {
    vec3  N     = normalize(vNormal);
    float slope = clamp(N.y, 0.0, 1.0);                  // 1 = flat, 0 = vertical
    float h     = clamp((vWorldPos.y - uTerrainMinY) /
                        max(uTerrainMaxY - uTerrainMinY, 1.0), 0.0, 1.0);

    // Topographic palette — height-driven for flats, slope override for cliffs.
    vec3 rock      = vec3(0.34, 0.33, 0.32);
    vec3 lowFlat   = vec3(0.78, 0.86, 0.84);             // frozen-grass / icy tint
    vec3 midPowder = vec3(0.92, 0.94, 0.97);
    vec3 highWhite = vec3(0.99, 0.99, 1.00);

    vec3 flatColor = mix(lowFlat,   midPowder, smoothstep(0.20, 0.55, h));
         flatColor = mix(flatColor, highWhite, smoothstep(0.65, 0.95, h));
    float rocky    = 1.0 - smoothstep(0.55, 0.78, slope);
    vec3  base     = mix(flatColor, rock, rocky);
    float snowness = 1.0 - rocky;

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

    // Per-fragment sparkle: high-freq world-cell hash gated by tight specular
    // alignment, drifting in time so cells flicker on/off without the camera
    // having to move. Snow-only.
    if (snowness > 0.0) {
        vec3  V    = normalize(uCameraPos - vWorldPos);
        vec3  H    = normalize(L + V);
        float spec = pow(max(dot(N, H), 0.0), 256.0);
        vec3  cell = floor(vWorldPos * 2.0 + uTime * 0.07); // ~50 cm cells
        float gate = step(0.985, hash3(cell));               // ~1.5 % of cells lit
        lit += gate * spec * snowness * vec3(1.4, 1.35, 1.2);
    }

    fragColor = vec4(lit, 1.0);

    // Overlay mode 1: contour lines (kept from b57bd9e).
    if (uOverlayMode == 1) {
        float elevMod = mod(vSmoothY, 50.0);
        float fw      = fwidth(vSmoothY);
        float line    = 1.0 - smoothstep(fw, fw * 3.0,
                                         min(elevMod, 50.0 - elevMod));
        fragColor.rgb = mix(fragColor.rgb, vec3(0.05, 0.05, 0.10), line * 0.85);
    }
    // Overlay mode 2: slope debug colouring.
    else if (uOverlayMode == 2) {
        vec3 slopeCol = mix(vec3(0.88, 0.15, 0.10),                       // red, steep
                            mix(vec3(0.93, 0.80, 0.08), vec3(0.15, 0.72, 0.20), // yellow → green
                                smoothstep(0.940, 0.975, slope)),
                            smoothstep(0.883, 0.940, slope));
        fragColor.rgb = mix(fragColor.rgb, slopeCol, 0.65);
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
