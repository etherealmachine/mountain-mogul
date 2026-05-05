#version 410 core

in vec2 vTexCoord;

uniform sampler2D uTexture;
uniform vec4 uColor;
uniform bool uUseTexture;

out vec4 fragColor;

void main() {
    if (uUseTexture) {
        fragColor = texture(uTexture, vTexCoord) * uColor;
    } else {
        fragColor = uColor;
    }
}
