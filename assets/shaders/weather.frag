#version 410 core

uniform float uTime;    // wall-clock seconds
uniform int   uWeather; // 0=clear/none, 1=overcast, 2=lightSnow, 3=heavySnow, 4=rain
uniform float uAspect;  // viewport width / height

in  vec2 vUV;   // (0,0)=top-left, (1,1)=bottom-right in screen space
out vec4 fragColor;

// ─── Hash ────────────────────────────────────────────────────────────────────

float hash21(vec2 p) {
    p = fract(p * vec2(127.1, 311.7));
    p += dot(p, p + 73.53);
    return fract(p.x * p.y);
}

// ─── Snow ────────────────────────────────────────────────────────────────────

// One layer of circular falling snowflakes on aspect-corrected UV.
// uv     — aspect-corrected: uv.x in [0, uAspect], uv.y in [0, 1]
// t      — time (seconds)
// scale  — grid cells per unit of uv
// speed  — fall speed in cells per second
// radius — flake radius in cell units
float snowLayer(vec2 uv, float t, float scale, float speed, float radius) {
    vec2 suv = uv * scale;
    // Gentle horizontal sway, varies per row so rows drift independently.
    suv.x += sin(t * 0.25 + floor(suv.y) * 1.3) * 0.20;

    vec2 id = floor(suv);
    vec2 f  = fract(suv);

    float fx = 0.15 + hash21(id) * 0.70;                         // fixed x per cell [0.15,0.85]
    float fy = fract(hash21(id + vec2(37.1, 19.3)) + t * speed); // y scrolls 0→1 (top→bottom)

    return smoothstep(radius, radius * 0.2, length(f - vec2(fx, fy)));
}

// ─── Rain ────────────────────────────────────────────────────────────────────

// One layer of thin vertical rain streaks on raw screen UV [0,1]×[0,1].
// suv   — raw screen UV (not aspect-corrected; preserves screen-proportional column width)
// t     — time (seconds)
// cols  — streak columns per screen width
// speed — head fall speed in screen-heights per second
// len   — streak body length as fraction of screen height
float rainLayer(vec2 suv, float t, float cols, float speed, float len) {
    float cx  = suv.x * cols;
    float cid = floor(cx);
    float fx  = fract(cx);

    float phase = hash21(vec2(cid, 7.3));
    float fy    = fract(phase * 5.37 + t * speed); // head y: 0=top, 1=bottom

    // delta > 0 means the fragment is above the head (in the streak body).
    float delta = fy - suv.y;
    if (delta < -0.5) delta += 1.0; // wrap when head is near top of screen

    float xFade = smoothstep(0.5, 0.20, abs(fx - 0.5)); // thin column
    float yFade = smoothstep(0.0, len * 0.7, delta)      // fade at tail
                * step(0.0, delta)                        // above head only
                * step(delta, len);                       // within body length

    return xFade * yFade;
}

// ─── Main ─────────────────────────────────────────────────────────────────────

void main() {
    // Wrap time to avoid float precision loss at large values.
    float t = mod(uTime, 200.0);

    // Aspect-corrected UV keeps snow flakes circular.
    vec2 uv = vec2(vUV.x * uAspect, vUV.y);

    vec4 result = vec4(0.0);

    if (uWeather == 2) {
        // ── Light snow: two gentle layers ──────────────────────────────
        float s1 = snowLayer(uv,             t,  7.0, 0.07, 0.065);
        float s2 = snowLayer(uv + vec2(0.31, 0.0), t, 14.0, 0.11, 0.040);
        float a  = clamp(s1 * 0.90 + s2 * 0.65, 0.0, 1.0);
        result   = vec4(1.0, 1.0, 1.0, a);

    } else if (uWeather == 3) {
        // ── Heavy snow: three layers + white fog veil ──────────────────
        float s1 = snowLayer(uv,                    t,  5.0, 0.12, 0.090);
        float s2 = snowLayer(uv + vec2(0.53, 0.0),  t, 11.0, 0.16, 0.056);
        float s3 = snowLayer(uv + vec2(1.07, 0.0),  t, 21.0, 0.22, 0.036);
        float flakes = clamp(s1 * 0.95 + s2 * 0.75 + s3 * 0.55, 0.0, 1.0);
        // Base fog gives a whiteout feel even between flakes.
        float a   = mix(0.18, 1.0, flakes * 0.90);
        vec3  col = mix(vec3(0.88, 0.90, 0.95), vec3(1.0), flakes);
        result    = vec4(col, clamp(a, 0.0, 1.0));

    } else if (uWeather == 4) {
        // ── Rain: two streak layers + slight grey haze ─────────────────
        float r1 = rainLayer(vUV,                    t, 70.0, 1.10, 0.10);
        float r2 = rainLayer(vUV + vec2(0.27, 0.0),  t, 50.0, 0.90, 0.07);
        float streaks = clamp(r1 * 0.85 + r2 * 0.55, 0.0, 1.0);
        vec3  col     = vec3(0.55, 0.65, 0.80);
        float a       = mix(0.07, 0.80, streaks);
        result        = vec4(col, clamp(a, 0.0, 1.0));
    }
    // uWeather 0 (clear) and 1 (overcast): sky colour alone does the work.

    fragColor = result;
}
