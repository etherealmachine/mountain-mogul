#version 410 core
layout(vertices = 3) out;

flat in vec3  vNormal[];
     in float vSmoothY[];
     in float vAO[];
     in vec4  vSnow[];
     in float vSnowDepth[];
     in vec3  vSmoothNormal[];
     in float vInstabilityScore[];

// vNormal is flat-shaded (per-primitive); promote to per-patch so TES can
// re-emit it as flat out without it varying across sub-triangles.
patch out vec3  tcNormal;
      out float tcSmoothY[];
      out float tcAO[];
      out vec4  tcSnow[];
      out float tcSnowDepth[];
      out vec3  tcSmoothNormal[];
      out float tcInstabilityScore[];

void main() {
    gl_out[gl_InvocationID].gl_Position  = gl_in[gl_InvocationID].gl_Position;
    tcSmoothY[gl_InvocationID]           = vSmoothY[gl_InvocationID];
    tcAO[gl_InvocationID]                = vAO[gl_InvocationID];
    tcSnow[gl_InvocationID]              = vSnow[gl_InvocationID];
    tcSnowDepth[gl_InvocationID]         = vSnowDepth[gl_InvocationID];
    tcSmoothNormal[gl_InvocationID]      = vSmoothNormal[gl_InvocationID];
    tcInstabilityScore[gl_InvocationID]  = vInstabilityScore[gl_InvocationID];

    if (gl_InvocationID == 0) {
        tcNormal = vNormal[0];
        gl_TessLevelInner[0] = 4.0;
        gl_TessLevelOuter[0] = 4.0;
        gl_TessLevelOuter[1] = 4.0;
        gl_TessLevelOuter[2] = 4.0;
    }
}
