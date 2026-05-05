#version 410 core

layout(location = 0) in vec3 aPos;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in vec2 aTexCoord;

// per-instance: mat4 occupies locations 3-6
layout(location = 3) in vec4 iTransform0;
layout(location = 4) in vec4 iTransform1;
layout(location = 5) in vec4 iTransform2;
layout(location = 6) in vec4 iTransform3;
layout(location = 7) in vec3 iColorTint;

uniform mat4 uViewProj;

out vec3 vColor;
flat out vec3 vNormal;
out vec2 vTexCoord;

void main() {
    mat4 iTransform = mat4(iTransform0, iTransform1, iTransform2, iTransform3);
    mat3 normalMatrix = mat3(transpose(inverse(mat3(iTransform))));
    vNormal = normalMatrix * aNormal;
    vColor = iColorTint;
    vTexCoord = aTexCoord;
    gl_Position = uViewProj * iTransform * vec4(aPos, 1.0);
}
