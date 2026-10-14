#version 410 core

// Water surfaces (render/water.go): open lakes and creeks, a flat sheet
// at the water's level over the open-water samples of the material map.

layout(location = 0) in vec3 aPos;
layout(location = 1) in vec3 aData; // (alpha: 1 on water, 0 on its rim; depth in metres; unused)

uniform mat4 uViewProj;

out vec3  vWorldPos;
out float vAlpha;
out float vDepth;

void main() {
    gl_Position = uViewProj * vec4(aPos, 1.0);
    vWorldPos = aPos;
    vAlpha = aData.x;
    vDepth = aData.y;
}
