#version 410 core

// Depth-only pass for dynamic instances (skiers, cats, chairs) into the
// sun's shadow map. Same heading rotation as dynamic.vert; rotor spin and
// limb bob are too small to show in a shadow and are skipped.
layout(location = 0) in vec3 aPos;
layout(location = 2) in vec3 iPosition;
layout(location = 3) in float iHeading;

uniform mat4 uLightVP;

void main() {
    float s = sin(iHeading);
    float c = cos(iHeading);
    vec3 p = vec3(s * aPos.x - c * aPos.z, aPos.y, c * aPos.x + s * aPos.z);
    gl_Position = uLightVP * vec4(p + iPosition, 1.0);
}
