#version 410 core

in vec3 vColor;
flat in vec3 vNormal;
in vec2 vTexCoord;
flat in float vPerceived;
in vec3 vWorldPos;

uniform sampler2D uTexture;
uniform float uAlpha;

out vec4 fragColor;

// lighting.glsl is prepended by shader loader

void main() {
    vec3 texColor = texture(uTexture, vTexCoord).rgb;
    vec3 baseColor = texColor * vColor;
    vec3 lit = computeLightingAt(vNormal, baseColor, vWorldPos);
    if (uLampCount > 0) {
        lit += baseColor * lampLight(vWorldPos, normalize(vNormal));
    }

    // Followed-skier perception highlight: warm yellow at ~50% mix.
    if (vPerceived > 0.5) {
        lit = mix(lit, vec3(1.0, 0.95, 0.1), 0.5);
    }

    fragColor = vec4(lit, uAlpha);
}
