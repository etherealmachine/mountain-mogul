vec3 computeLighting(vec3 normal, vec3 baseColor) {
    vec3 lightDir = normalize(vec3(0.6, 1.0, 0.4));
    float diff = max(dot(normalize(normal), lightDir), 0.0);
    vec3 ambient = 0.25 * baseColor;
    vec3 diffuse = diff * baseColor;
    return ambient + diffuse;
}
