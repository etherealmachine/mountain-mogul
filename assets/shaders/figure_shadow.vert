#version 410 core

// Depth-only pass for jointed figures into the sun's shadow map: the
// same bone lookup as figure.vert.
layout(location = 0) in vec3 aPos;
layout(location = 2) in float aBone;

uniform samplerBuffer uFigures;
uniform mat4 uLightVP;

const int STRIDE = 17 * 3 + 5 + 1;

void main() {
    int b = gl_InstanceID * STRIDE + int(aBone + 0.5) * 3;
    vec4 p = vec4(aPos, 1.0);
    vec3 world = vec3(dot(texelFetch(uFigures, b), p), dot(texelFetch(uFigures, b + 1), p), dot(texelFetch(uFigures, b + 2), p));
    gl_Position = uLightVP * vec4(world, 1.0);
}
