#version 410 core

in vec3 vColor;
flat in vec3 vNormal;
in vec2 vTexCoord;

uniform sampler2D uTexture;
uniform float uAlpha;

out vec4 fragColor;

// lighting.glsl is prepended by shader loader

void main() {
    vec3 texColor = texture(uTexture, vTexCoord).rgb;
    vec3 baseColor = texColor * vColor;
    vec3 lit = computeLighting(vNormal, baseColor);
    fragColor = vec4(lit, uAlpha);
}
