#version 410 core

layout(location = 0) in vec3  aPos;
layout(location = 1) in vec3  aNormal;   // per-triangle face normal (flat shaded)
layout(location = 2) in float aSmoothY;  // low-pass filtered elevation, for contour overlay
layout(location = 3) in float aAO;       // baked vertex AO in [0, 1]

uniform mat4 uViewProj;

flat out vec3  vNormal;
out vec3  vWorldPos;
out float vSmoothY;
out float vAO;

void main() {
    vNormal     = aNormal;
    vWorldPos   = aPos;
    vSmoothY    = aSmoothY;
    vAO         = aAO;
    gl_Position = uViewProj * vec4(aPos, 1.0);
}
