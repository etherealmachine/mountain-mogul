#version 410 core

// Depth-only pass for dynamic instances (skiers, cats, chairs) into the
// sun's shadow map. Same heading rotation and scale as dynamic.vert;
// rotor spin and limb bob are too small to show in a shadow and are
// skipped.
layout(location = 0) in vec3 aPos;
layout(location = 2) in vec3 iPosition;
layout(location = 3) in float iHeading;
layout(location = 5) in float iSpinMode;
layout(location = 6) in float iPitch;

uniform mat4 uLightVP;

void main() {
    float s = sin(iHeading);
    float c = cos(iHeading);
    float sp = sin(iPitch);
    float cp = cos(iPitch);
    // 10 and up: scaled by iSpinMode - 10 (debris blocks, avalanche.go)
    vec3 a = iSpinMode > 9.5 ? aPos * (iSpinMode - 10.0) : aPos;
    vec3 q = vec3(cp * a.x - sp * a.y, sp * a.x + cp * a.y, a.z);
    vec3 p = vec3(s * q.x - c * q.z, q.y, c * q.x + s * q.z);
    gl_Position = uLightVP * vec4(p + iPosition, 1.0);
}
