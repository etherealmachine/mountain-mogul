#version 410 core

layout(location = 0) in vec3 aPos;
layout(location = 1) in vec3 aNormal;

// per-instance
layout(location = 2) in vec3 iPosition;
layout(location = 3) in float iHeading;
layout(location = 4) in vec3 iColor;

// per-vertex base colour from the 3MF pipeline (color() blocks in SCAD).
// Meshes without per-vertex colour leave this unbound — the renderer sets
// a constant default of (1,1,1) so chairs and agent boxes tint via iColor.
layout(location = 8) in vec3 aBaseColor;

uniform mat4 uViewProj;
uniform float uTime;

out vec3 vColor;
flat out vec3 vNormal;

void main() {
    float s = sin(iHeading);
    float c = cos(iHeading);
    mat4 rotY = mat4(
        c,   0.0, s,   0.0,
        0.0, 1.0, 0.0, 0.0,
        -s,  0.0, c,   0.0,
        0.0, 0.0, 0.0, 1.0
    );
    mat4 translate = mat4(
        1.0, 0.0, 0.0, 0.0,
        0.0, 1.0, 0.0, 0.0,
        0.0, 0.0, 1.0, 0.0,
        iPosition.x, iPosition.y, iPosition.z, 1.0
    );

    // limb animation: small Y displacement for vertices above threshold
    float phase = sin(uTime * 3.0 + float(gl_InstanceID) * 1.618);
    float animY = 0.0;
    if (aPos.y > 0.3) {
        animY = phase * 0.05;
    }

    vec3 animPos = vec3(aPos.x, aPos.y + animY, aPos.z);
    mat4 model = translate * rotY;

    vColor = iColor * aBaseColor;
    vNormal = mat3(rotY) * aNormal;
    gl_Position = uViewProj * model * vec4(animPos, 1.0);
}
