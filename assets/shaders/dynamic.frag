#version 410 core

in vec3 vColor;
flat in vec3 vNormal;

out vec4 fragColor;

// lighting.glsl is prepended by shader loader

void main() {
    vec3 lit = computeLighting(vNormal, vColor);
    fragColor = vec4(lit, 1.0);
}
