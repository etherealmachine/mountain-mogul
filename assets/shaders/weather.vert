#version 410 core

out vec2 vUV;

void main() {
    // Full-screen triangle — covers NDC [-1,1]² with three vertices, no VBO.
    // VertexID 0 → (-1,-1), 1 → (3,-1), 2 → (-1,3).
    float x = float((gl_VertexID & 1) << 2) - 1.0;
    float y = float((gl_VertexID >> 1) << 2) - 1.0;
    // vUV: (0,0) at screen top-left, (1,1) at screen bottom-right.
    vUV = vec2(x * 0.5 + 0.5, -y * 0.5 + 0.5);
    gl_Position = vec4(x, y, 0.0, 1.0);
}
