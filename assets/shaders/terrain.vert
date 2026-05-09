#version 410 core

layout(location = 0) in vec3 aPos;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in vec3 aSmoothNormal;
layout(location = 3) in vec3 aColor;
layout(location = 4) in float aSmoothY;

uniform mat4 uViewProj;

out vec3 vColor;
flat out vec3 vNormal;
out vec3 vSmoothNormal;
out vec3 vWorldPos;
out float vSmoothY;

void main() {
    vColor        = aColor;
    vNormal       = aNormal;
    vSmoothNormal = aSmoothNormal;
    vWorldPos     = aPos;
    vSmoothY      = aSmoothY;
    gl_Position   = uViewProj * vec4(aPos, 1.0);
}
