#version 410 core

in vec3 vColor;

// uTint scales the colour: 1 for debug overlays, the frame's light level
// for things in the world (fence posts, boundary rope) so they dim at
// night instead of glowing.
uniform vec3 uTint;

out vec4 outColor;

void main() {
    outColor = vec4(vColor * uTint, 1.0);
}
