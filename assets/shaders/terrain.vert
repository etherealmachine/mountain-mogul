#version 410 core

layout(location = 0) in vec3  aPos;          // jittered corner XZ, ground Y
layout(location = 1) in vec2  aGrid;         // corner-grid coordinate
layout(location = 2) in vec2  aKind;         // (detail weight, wall flag)
layout(location = 3) in float aSmoothY;      // low-pass filtered elevation, for contour overlay
layout(location = 4) in float aAO;           // baked vertex AO in [0, 1]
layout(location = 5) in vec3  aSmoothNormal; // per-corner smoothed normal; the face normal on walls

// Per-corner snow state, one texel per corner (see terrain_mesh.go):
//   A = (Grooming, Packed, Ice, MogulSize), B = (visible depth m, instability)
uniform sampler2D uCornerSnowA;
uniform sampler2D uCornerSnowB;

out vec2  vGrid;
out vec2  vKind;
out float vSmoothY;
out float vAO;
out vec4  vSnow;
out float vSnowDepth;
out vec3  vSmoothNormal;
out float vInstabilityScore;

void main() {
    vGrid         = aGrid;
    vKind         = aKind;
    vSmoothY      = aSmoothY;
    vAO           = aAO;
    vSmoothNormal = aSmoothNormal;
    if (aKind.y > 0.5) {
        // Map-edge skirt: no snow; -1 instability marks it for terrain.frag.
        vSnow             = vec4(0.0);
        vSnowDepth        = 0.0;
        vInstabilityScore = -1.0;
    } else {
        ivec2 c           = ivec2(aGrid + 0.5);
        vSnow             = texelFetch(uCornerSnowA, c, 0);
        vec2  b           = texelFetch(uCornerSnowB, c, 0).rg;
        vSnowDepth        = b.x;
        vInstabilityScore = b.y;
    }
    // World-space position — TES applies displacement then projects.
    gl_Position = vec4(aPos, 1.0);
}
