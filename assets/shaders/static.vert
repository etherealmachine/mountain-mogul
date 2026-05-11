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

// per-vertex base colour from the 3MF pipeline (color() blocks in SCAD).
// Meshes without per-vertex colour leave this unbound — the renderer sets
// a constant default of (1,1,1) so they tint via iColorTint alone.
layout(location = 8) in vec3 aBaseColor;

uniform mat4 uViewProj;

// Followed-skier perception fan. uPerceptionRadius == 0 disables.
uniform vec3  uPerceptionOrigin;
uniform vec2  uPerceptionForwardXZ;   // unit (sin(h), cos(h))
uniform float uPerceptionCosHalfAngle;
uniform float uPerceptionRadius;

out vec3 vColor;
flat out vec3 vNormal;
out vec2 vTexCoord;
flat out float vPerceived;

void main() {
    mat4 iTransform = mat4(iTransform0, iTransform1, iTransform2, iTransform3);
    mat3 normalMatrix = mat3(transpose(inverse(mat3(iTransform))));
    vNormal = normalMatrix * aNormal;
    vColor = iColorTint * aBaseColor;
    vTexCoord = aTexCoord;

    // Per-instance perception-fan test. Trees are point-instanced, so testing
    // the instance origin (translation column of iTransform) gives a uniform
    // glow on the whole tree — easier to read than a per-fragment bisection.
    vPerceived = 0.0;
    if (uPerceptionRadius > 0.0) {
        vec2 d = iTransform3.xz - uPerceptionOrigin.xz;
        float dist = length(d);
        if (dist <= uPerceptionRadius && dist > 0.0001) {
            float cosA = dot(d / dist, uPerceptionForwardXZ);
            vPerceived = step(uPerceptionCosHalfAngle, cosA);
        }
    }

    gl_Position = uViewProj * iTransform * vec4(aPos, 1.0);
}
