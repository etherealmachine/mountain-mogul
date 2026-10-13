#version 410 core

// Menu thumbnails: one model part drawn without instancing (thumbnails.go).

layout(location = 0) in vec3 aPos;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in vec2 aTexCoord;
layout(location = 8) in vec3 aBaseColor;

uniform mat4 uViewProj;
uniform mat4 uModel;
uniform vec3 uTint;

out vec3 vColor;
out vec3 vNormal;
out vec2 vTexCoord;

void main() {
    vNormal = mat3(uModel) * aNormal;
    vColor = uTint * aBaseColor;
    vTexCoord = aTexCoord;
    gl_Position = uViewProj * uModel * vec4(aPos, 1.0);
}
