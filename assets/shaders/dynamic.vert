#version 410 core

layout(location = 0) in vec3 aPos;
layout(location = 1) in vec3 aNormal;

// per-instance
layout(location = 2) in vec3 iPosition;
layout(location = 3) in float iHeading;
layout(location = 4) in vec3 iColor;
layout(location = 5) in float iSpinMode;
// 0 = rigid  2 = spin_y (main rotor)  3 = spin_z (tail rotor)

// per-vertex base colour from the 3MF pipeline (color() blocks in SCAD).
// Meshes without per-vertex colour leave this unbound — the renderer sets
// a constant default of (1,1,1) so chairs and agent boxes tint via iColor.
layout(location = 8) in vec3 aBaseColor;

uniform mat4 uViewProj;
uniform float uTime;
uniform float uSpinRate; // rad/s for spin_y / spin_z modes; 0 for all others

out vec3 vColor;
flat out vec3 vNormal;
out vec3 vWorldPos;

void main() {
    // Heading is computed as atan2(dx, dz) — angle from +Z toward +X —
    // so motion direction in world space is (sin h, 0, cos h). The rotation
    // must take the model's +X axis (the project's "forward" convention,
    // matching snowcat.scad's +X = direction of travel) to that motion
    // vector, with +Z mapped 90° CCW (left of forward) for consistency.
    float s = sin(iHeading);
    float c = cos(iHeading);
    mat4 rotY = mat4(
        s,   0.0, c,   0.0,
        0.0, 1.0, 0.0, 0.0,
        -c,  0.0, s,   0.0,
        0.0, 0.0, 0.0, 1.0
    );
    mat4 translate = mat4(
        1.0, 0.0, 0.0, 0.0,
        0.0, 1.0, 0.0, 0.0,
        0.0, 0.0, 1.0, 0.0,
        iPosition.x, iPosition.y, iPosition.z, 1.0
    );

    // Rotor spin: rotate vertices around local origin before heading rotation.
    // Each rotor OBJ has its pivot at origin so no per-instance pivot needed.
    vec3 spinPos = aPos;
    if (iSpinMode > 1.5) {
        float a = uTime * uSpinRate;
        float ss = sin(a);
        float cs = cos(a);
        if (iSpinMode < 2.5) {
            // spin_y — main rotor: vertical axis
            spinPos = vec3(aPos.x * cs - aPos.z * ss, aPos.y, aPos.x * ss + aPos.z * cs);
        } else {
            // spin_z — tail rotor: shaft (fuselage-forward) axis
            spinPos = vec3(aPos.x * cs - aPos.y * ss, aPos.x * ss + aPos.y * cs, aPos.z);
        }
    }

    vec3 animPos = spinPos;
    mat4 model = translate * rotY;

    vColor = iColor * aBaseColor;
    vNormal = mat3(rotY) * aNormal;
    vec4 world = model * vec4(animPos, 1.0);
    vWorldPos = world.xyz;
    gl_Position = uViewProj * world;
}
