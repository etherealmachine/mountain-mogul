// Scene lighting, set per frame from the sim clock (render.Lighting).
uniform vec3 uSunDir;   // unit vector toward the key light: the sun by day, the moon at night
uniform vec3 uSunColor; // key light colour × intensity; ~1 at clear noon, dim blue by moonlight
uniform vec3 uAmbient;  // sky fill light colour × intensity

vec3 computeLighting(vec3 normal, vec3 baseColor) {
    float diff = max(dot(normalize(normal), uSunDir), 0.0);
    return baseColor * (uAmbient + diff * uSunColor);
}

// sceneLight is the overall light level (1 ≈ clear midday), for colours
// that are painted on rather than lit.
vec3 sceneLight() {
    return (uAmbient + 0.85 * uSunColor) / 1.1;
}
