#version 410 core

// Jointed figures (render/figure.go): each vertex belongs to one bone and
// takes its colour from one slot. Per instance, uFigures holds STRIDE
// texels: three rows of each bone's model-to-world matrix, the outfit
// colours, then a tint (rgb, mix amount).
layout(location = 0) in vec3 aPos;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in float aBone;
layout(location = 3) in float aSlot;

uniform samplerBuffer uFigures;
uniform mat4 uViewProj;

out vec3 vColor;
flat out vec3 vNormal;
out vec3 vWorldPos;

const int BONES = 17;
const int OUTFIT = 5;
const int STRIDE = BONES * 3 + OUTFIT + 1;

// Fixed colours for slots past the outfit: boots, gloves, goggles, poles.
const vec3 FIXED[4] = vec3[4](
    vec3(0.12, 0.12, 0.14),
    vec3(0.08, 0.08, 0.09),
    vec3(0.20, 0.35, 0.55),
    vec3(0.70, 0.72, 0.75)
);

void main() {
    int base = gl_InstanceID * STRIDE;
    int b = base + int(aBone + 0.5) * 3;
    vec4 r0 = texelFetch(uFigures, b);
    vec4 r1 = texelFetch(uFigures, b + 1);
    vec4 r2 = texelFetch(uFigures, b + 2);
    vec4 p = vec4(aPos, 1.0);
    vec3 world = vec3(dot(r0, p), dot(r1, p), dot(r2, p));
    vNormal = normalize(vec3(dot(r0.xyz, aNormal), dot(r1.xyz, aNormal), dot(r2.xyz, aNormal)));

    int slot = int(aSlot + 0.5);
    vec3 col = slot < OUTFIT ? texelFetch(uFigures, base + BONES * 3 + slot).rgb : FIXED[slot - OUTFIT];
    vec4 tint = texelFetch(uFigures, base + BONES * 3 + OUTFIT);
    vColor = mix(col, tint.rgb, tint.a);

    vWorldPos = world;
    gl_Position = uViewProj * vec4(world, 1.0);
}
