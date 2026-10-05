#version 410 core
layout(vertices = 3) out;

uniform mat4  uViewProj;
uniform vec2  uViewport; // logical pixels
uniform float uTessPx;   // target on-screen length of one subdivided segment
uniform float uTessMax;

in vec2  vGrid[];
in vec2  vKind[];
in float vSmoothY[];
in float vAO[];
in vec4  vSnow[];
in float vSnowDepth[];
in vec3  vSmoothNormal[];
in float vInstabilityScore[];

// The flat face normal, per patch so TES re-emits it unchanged across
// sub-triangles.
patch out vec3  tcNormal;
      out vec2  tcGrid[];
      out vec2  tcKind[];
      out float tcSmoothY[];
      out float tcAO[];
      out vec4  tcSnow[];
      out float tcSnowDepth[];
      out vec3  tcSmoothNormal[];
      out float tcInstabilityScore[];

// edgeLevel depends only on the edge's two endpoints, so the two patches
// sharing an edge always agree and the surface can't crack.
float edgeLevel(vec3 a, vec3 b) {
    vec4 ca = uViewProj * vec4(a, 1.0);
    vec4 cb = uViewProj * vec4(b, 1.0);
    if (ca.w <= 1e-4 || cb.w <= 1e-4) {
        return uTessMax;
    }
    vec2 px = (ca.xy / ca.w - cb.xy / cb.w) * 0.5 * uViewport;
    return clamp(ceil(length(px) / uTessPx), 1.0, uTessMax);
}

void main() {
    gl_out[gl_InvocationID].gl_Position = gl_in[gl_InvocationID].gl_Position;
    tcGrid[gl_InvocationID]             = vGrid[gl_InvocationID];
    tcKind[gl_InvocationID]             = vKind[gl_InvocationID];
    tcSmoothY[gl_InvocationID]          = vSmoothY[gl_InvocationID];
    tcAO[gl_InvocationID]               = vAO[gl_InvocationID];
    tcSnow[gl_InvocationID]             = vSnow[gl_InvocationID];
    tcSnowDepth[gl_InvocationID]        = vSnowDepth[gl_InvocationID];
    tcSmoothNormal[gl_InvocationID]     = vSmoothNormal[gl_InvocationID];
    tcInstabilityScore[gl_InvocationID] = vInstabilityScore[gl_InvocationID];

    if (gl_InvocationID == 0) {
        vec3 p0 = gl_in[0].gl_Position.xyz;
        vec3 p1 = gl_in[1].gl_Position.xyz;
        vec3 p2 = gl_in[2].gl_Position.xyz;
        if (vKind[0].y > 0.5) {
            tcNormal = vSmoothNormal[0];
        } else {
            vec3 n = normalize(cross(p1 - p0, p2 - p0));
            tcNormal = n.y < 0.0 ? -n : n;
        }
        float e0 = edgeLevel(p1, p2);
        float e1 = edgeLevel(p2, p0);
        float e2 = edgeLevel(p0, p1);
        gl_TessLevelOuter[0] = e0;
        gl_TessLevelOuter[1] = e1;
        gl_TessLevelOuter[2] = e2;
        gl_TessLevelInner[0] = max(e0, max(e1, e2));
    }
}
