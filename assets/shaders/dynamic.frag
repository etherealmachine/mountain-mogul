#version 410 core

in vec3 vColor;
flat in vec3 vNormal;
in vec3 vWorldPos;

out vec4 fragColor;

// uEmissive = 1 draws vColor unlit (lamp lenses, beacons).
uniform float uEmissive;

// lighting.glsl is prepended by shader loader

void main() {
    if (uEmissive > 0.5) {
        fragColor = vec4(vColor, 1.0);
        return;
    }
    vec3 lit = computeLightingAt(vNormal, vColor, vWorldPos);
    fragColor = vec4(toneMap(applyHaze(lit, vWorldPos)), 1.0);
}
