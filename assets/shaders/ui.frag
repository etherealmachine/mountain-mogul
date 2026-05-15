#version 410 core

in vec2 vTexCoord;
in vec4 vColor;

uniform sampler2D uTexture;

out vec4 fragColor;

// All UI quads sample uTexture and multiply by vColor. Color-only quads
// bind the 1×1 white fallback so sampling returns vec4(1) and the output
// reduces to vColor. Same shader path for textured and untextured — no
// per-draw uniform toggling, so the renderer can batch arbitrary
// sequences of quads into one DrawArrays call.
void main() {
    fragColor = texture(uTexture, vTexCoord) * vColor;
}
