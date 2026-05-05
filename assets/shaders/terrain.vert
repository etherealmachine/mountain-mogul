#version 410 core

layout(location = 0) in vec3 aPos;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in vec3 aColor;

uniform mat4 uViewProj;

out vec3 vColor;
flat out vec3 vNormal;
out vec2 vWorldXZ;

void main() {
    vColor = aColor;
    vNormal = aNormal;
    vWorldXZ = aPos.xz;
    gl_Position = uViewProj * vec4(aPos, 1.0);
}
