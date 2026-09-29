#version 410 core

// Depth-only pass for static instances (trees, buildings, lift towers)
// into the sun's shadow map. Same instance layout as static.vert.
layout(location = 0) in vec3 aPos;
layout(location = 3) in vec4 iTransform0;
layout(location = 4) in vec4 iTransform1;
layout(location = 5) in vec4 iTransform2;
layout(location = 6) in vec4 iTransform3;

uniform mat4 uLightVP;

void main() {
    mat4 iTransform = mat4(iTransform0, iTransform1, iTransform2, iTransform3);
    gl_Position = uLightVP * iTransform * vec4(aPos, 1.0);
}
