// Scene lighting, set per frame from the sim clock (render.Lighting).
uniform vec3 uSunDir;   // unit vector toward the key light: the sun by day, the moon at night
uniform vec3 uSunColor; // key light colour × intensity; ~1 at clear noon, dim blue by moonlight
uniform vec3 uAmbient;    // sky fill from above, colour × intensity
uniform vec3 uGroundFill; // bounce fill from below (sunlit snow)

// fillLight is the ambient reaching a surface facing n: sky from above,
// bounce from the ground below.
vec3 fillLight(vec3 n) {
    return mix(uGroundFill, uAmbient, clamp(normalize(n).y * 0.5 + 0.5, 0.0, 1.0));
}

// toneMap leaves colours below the shoulder alone and rolls brighter
// values off toward 1, so sunlit snow reads white without clipping.
vec3 toneMap(vec3 c) {
    const float knee = 0.85;
    vec3 over = max(c - knee, 0.0);
    return min(c, knee) + (1.0 - knee) * (1.0 - exp(-over / (1.0 - knee)));
}

// Terrain shadow: per-cell visibility of the key light past ridges
// (world.HorizonMap), one texel per 5 m cell. uSunVisOn = 0 → fully lit.
uniform sampler2D uSunVis;
uniform vec2 uSunVisWorld; // terrain extent in metres
uniform int uSunVisOn;

float sunVisibility(vec3 p) {
    if (uSunVisOn == 0) {
        return 1.0;
    }
    return texture(uSunVis, p.xz / uSunVisWorld).r;
}

// Object shadows: a directional shadow map from the key light
// (render/shadow_map.go). Trees, buildings, lifts and vehicles cast.
uniform sampler2DShadow uShadowMap;
uniform mat4  uShadowVP;
uniform int   uShadowOn;
uniform float uShadowTexelWorld; // metres per shadow-map texel

float objectShadow(vec3 p, vec3 n) {
    if (uShadowOn == 0) {
        return 1.0;
    }
    // Normal offset: push the lookup off the surface by ~1.5 texels so
    // surfaces don't shadow themselves.
    vec3 q = p + normalize(n) * uShadowTexelWorld * 1.5;
    vec3 c = (uShadowVP * vec4(q, 1.0)).xyz * 0.5 + 0.5;
    if (c.z >= 1.0) {
        return 1.0;
    }
    // 4 bilinear-PCF taps half a texel apart ≈ a 3×3 soft kernel.
    vec2 t = vec2(1.0 / textureSize(uShadowMap, 0).x);
    float s = texture(uShadowMap, vec3(c.xy + vec2(-0.5, -0.5) * t, c.z))
            + texture(uShadowMap, vec3(c.xy + vec2( 0.5, -0.5) * t, c.z))
            + texture(uShadowMap, vec3(c.xy + vec2(-0.5,  0.5) * t, c.z))
            + texture(uShadowMap, vec3(c.xy + vec2( 0.5,  0.5) * t, c.z));
    return s * 0.25;
}

// keyLightVisibility is how much of the key light reaches p: blocked by
// ridges (horizon map) or by objects (shadow map).
float keyLightVisibility(vec3 p, vec3 n) {
    return sunVisibility(p) * objectShadow(p, n);
}

vec3 computeLighting(vec3 normal, vec3 baseColor) {
    float diff = max(dot(normalize(normal), uSunDir), 0.0);
    return baseColor * (fillLight(normal) + diff * uSunColor);
}

// computeLightingAt is computeLighting for a surface at world position p,
// shaded when terrain blocks the key light.
vec3 computeLightingAt(vec3 normal, vec3 baseColor, vec3 p) {
    float diff = max(dot(normalize(normal), uSunDir), 0.0) * keyLightVisibility(p, normal);
    return baseColor * (fillLight(normal) + diff * uSunColor);
}

// Vehicle lamps (snowcat headlights and work lights): spotlights that
// light the snow and trees around working machines after dark.
// uLampCount = 0 in daylight, so the loop costs nothing then.
#define MAX_LAMPS 16
uniform int  uLampCount;
uniform vec4 uLampPos[MAX_LAMPS];   // xyz world position, w = range (m)
uniform vec4 uLampDir[MAX_LAMPS];   // xyz unit beam direction, w = cos(outer half-angle)
uniform vec3 uLampColor[MAX_LAMPS]; // colour × intensity

vec3 lampLight(vec3 p, vec3 n) {
    vec3 sum = vec3(0.0);
    for (int i = 0; i < uLampCount; i++) {
        vec3 toLamp = uLampPos[i].xyz - p;
        float d = length(toLamp);
        float range = uLampPos[i].w;
        if (d >= range) {
            continue;
        }
        vec3 l = toLamp / d;
        float cosOuter = uLampDir[i].w;
        float cone = smoothstep(cosOuter, mix(cosOuter, 1.0, 0.5), dot(-l, uLampDir[i].xyz));
        float fall = 1.0 - d / range;
        sum += uLampColor[i] * cone * fall * fall * max(dot(n, l), 0.0);
    }
    return sum;
}

// Atmospheric haze: thickens with distance beyond the camera's focus and
// pools in the low valleys of the map. uHazeOn = 0 disables.
uniform int  uHazeOn;
uniform vec3 uHazeOrigin;  // camera focus point
uniform vec3 uHazeForward; // unit view direction
uniform vec2 uHazeLowHigh; // terrain min / max elevation

vec3 sceneLight();

vec3 applyHaze(vec3 c, vec3 p) {
    if (uHazeOn == 0) {
        return c;
    }
    float dist   = max(dot(p - uHazeOrigin, uHazeForward), 0.0);
    float far    = 1.0 - exp(-dist / 2500.0);
    float relief = max(uHazeLowHigh.y - uHazeLowHigh.x, 1.0);
    float low    = 1.0 - smoothstep(0.0, 0.35, (p.y - uHazeLowHigh.x) / relief);
    float f      = clamp(far * 0.5 + low * 0.10, 0.0, 0.35);
    return mix(c, sceneLight() * vec3(0.82, 0.87, 0.95), f);
}

// sceneLight is the overall light level (1 ≈ clear midday), for colours
// that are painted on rather than lit.
vec3 sceneLight() {
    return (uAmbient + 0.85 * uSunColor) / 1.1;
}
