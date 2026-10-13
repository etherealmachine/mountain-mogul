#version 410 core

// Menu thumbnails: a bright, even studio light so small pictures read.

in vec3 vColor;
in vec3 vNormal;
in vec2 vTexCoord;

uniform sampler2D uTexture;
uniform vec3 uLightDir; // towards the key light, world space

out vec4 fragColor;

void main() {
    vec3 N = normalize(vNormal);
    if (!gl_FrontFacing) N = -N;
    vec3 base = texture(uTexture, vTexCoord).rgb * vColor;
    float key = max(dot(N, uLightDir), 0.0);
    float sky = 0.5 + 0.5 * N.y;
    vec3 c = base * (0.55 + 0.30 * sky + 0.75 * key);
    // Soft shoulder, as the world's toneMap, so whites don't clip.
    vec3 over = max(c - 0.85, 0.0);
    c = min(c, 0.85) + 0.15 * (1.0 - exp(-over / 0.15));
    fragColor = vec4(c, 1.0);
}
