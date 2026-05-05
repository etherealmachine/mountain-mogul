#version 410 core

in vec3 vColor;
flat in vec3 vNormal;
in vec2 vWorldXZ;

uniform vec2  uBrushCenter;
uniform float uBrushRadius;

out vec4 fragColor;

// lighting.glsl is prepended by shader loader

void main() {
    vec3 lit = computeLighting(vNormal, vColor);
    fragColor = vec4(lit, 1.0);

    if (uBrushRadius > 0.0) {
        float d    = length(vWorldXZ - uBrushCenter);
        float ring = abs(d - uBrushRadius);
        float t    = 1.0 - clamp(ring / 5.0, 0.0, 1.0);
        fragColor  = mix(fragColor, vec4(1.0, 1.0, 0.3, 1.0), t * 0.85);
        if (d < uBrushRadius - 5.0) {
            fragColor = mix(fragColor, vec4(1.0, 1.0, 0.5, 1.0), 0.12);
        }
    }
}
