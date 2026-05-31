#version 410 core

layout(location = 0) in vec3  aPos;
layout(location = 1) in vec3  aNormal;       // per-triangle face normal (flat shaded)
layout(location = 2) in float aSmoothY;      // low-pass filtered elevation, for contour overlay
layout(location = 3) in float aAO;           // baked vertex AO in [0, 1]
layout(location = 4) in vec4  aSnow;         // (Grooming, Packed, Ice, MogulSize) per cell-corner
layout(location = 5) in float aSnowDepth;    // SnowDepth in metres, per cell-corner
layout(location = 6) in vec3  aSmoothNormal;      // per-corner smoothed normal (non-flat varying)
layout(location = 7) in float aInstabilityScore;  // Cell.InstabilityScore(); 0=stable, ≥1=release threshold

uniform mat4 uViewProj;

flat out vec3  vNormal;
out vec3  vWorldPos;
out float vSmoothY;
out float vAO;
out vec4  vSnow;
out float vSnowDepth;
out vec3  vSmoothNormal;
out float vInstabilityScore;

void main() {
    vNormal            = aNormal;
    vWorldPos          = aPos;
    vSmoothY           = aSmoothY;
    vAO                = aAO;
    vSnow              = aSnow;
    vSnowDepth         = aSnowDepth;
    vSmoothNormal      = aSmoothNormal;
    vInstabilityScore  = aInstabilityScore;
    gl_Position        = uViewProj * vec4(aPos, 1.0);
}
