"""Render model OBJs to a PNG for a look without the game: a 3/4 view
from the front left, flat shaded in the models' own colours, several
models side by side on one sheet.

    .venv-models/bin/python tools/modelview/view.py out.png a.obj b.obj ...

YAW and PITCH (degrees, env) turn the view; SIZE sets each tile's pixels.
"""
import math
import os
import sys

import numpy as np
from PIL import Image, ImageDraw


def load(path):
    vs, cols, faces = [], [], []
    for line in open(path):
        if line.startswith("v "):
            f = [float(x) for x in line.split()[1:]]
            vs.append(f[:3])
            cols.append(f[3:6] if len(f) >= 6 else [0.8, 0.8, 0.8])
        elif line.startswith("f "):
            idx = [int(p.split("/")[0]) - 1 for p in line.split()[1:]]
            for i in range(1, len(idx) - 1):
                faces.append((idx[0], idx[i], idx[i + 1]))
    return np.array(vs), np.array(cols), faces


def render(path, size, yaw, pitch):
    v, c, faces = load(path)
    # Game space is Y-up; view from yaw about Y, then pitch down.
    cy, sy = math.cos(math.radians(yaw)), math.sin(math.radians(yaw))
    cp, sp = math.cos(math.radians(pitch)), math.sin(math.radians(pitch))
    x = v[:, 0] * cy - v[:, 2] * sy
    z = v[:, 0] * sy + v[:, 2] * cy
    y = v[:, 1]
    y2 = y * cp - z * sp
    z2 = y * sp + z * cp
    lo, hi = np.array([x.min(), y2.min()]), np.array([x.max(), y2.max()])
    scale = 0.9 * size / max(hi - lo)
    px = (x - (lo[0] + hi[0]) / 2) * scale + size / 2
    py = size / 2 - (y2 - (lo[1] + hi[1]) / 2) * scale
    img = np.full((size, size, 3), 235, dtype=np.uint8)
    zbuf = np.full((size, size), -1e9)
    light = np.array([0.4, 0.8, 0.45])
    light /= np.linalg.norm(light)
    for a, b, d in faces:
        p0, p1, p2 = v[a], v[b], v[d]
        n = np.cross(p1 - p0, p2 - p0)
        ln = np.linalg.norm(n)
        if ln == 0:
            continue
        shade = 0.35 + 0.65 * abs(np.dot(n / ln, light))
        col = np.clip(c[a] * shade * 255, 0, 255)
        xs, ys, zs = px[[a, b, d]], py[[a, b, d]], z2[[a, b, d]]
        x0, x1 = int(max(min(xs), 0)), int(min(max(xs) + 1, size - 1))
        y0, y1 = int(max(min(ys), 0)), int(min(max(ys) + 1, size - 1))
        if x1 < x0 or y1 < y0:
            continue
        gx, gy = np.meshgrid(np.arange(x0, x1 + 1) + 0.5, np.arange(y0, y1 + 1) + 0.5)
        den = (ys[1] - ys[2]) * (xs[0] - xs[2]) + (xs[2] - xs[1]) * (ys[0] - ys[2])
        if abs(den) < 1e-9:
            continue
        w0 = ((ys[1] - ys[2]) * (gx - xs[2]) + (xs[2] - xs[1]) * (gy - ys[2])) / den
        w1 = ((ys[2] - ys[0]) * (gx - xs[2]) + (xs[0] - xs[2]) * (gy - ys[2])) / den
        w2 = 1 - w0 - w1
        inside = (w0 >= 0) & (w1 >= 0) & (w2 >= 0)
        depth = w0 * zs[0] + w1 * zs[1] + w2 * zs[2]
        sub = zbuf[y0:y1 + 1, x0:x1 + 1]
        m = inside & (depth > sub)
        sub[m] = depth[m]
        img[y0:y1 + 1, x0:x1 + 1][m] = col
    return Image.fromarray(img)


def main():
    out, paths = sys.argv[1], sys.argv[2:]
    size = int(os.environ.get("SIZE", "420"))
    yaw = float(os.environ.get("YAW", "35"))
    pitch = float(os.environ.get("PITCH", "20"))
    sheet = Image.new("RGB", (size * len(paths), size + 18), (255, 255, 255))
    draw = ImageDraw.Draw(sheet)
    for i, p in enumerate(paths):
        sheet.paste(render(p, size, yaw, pitch), (i * size, 0))
        draw.text((i * size + 6, size + 3), os.path.basename(p), fill=(0, 0, 0))
    sheet.save(out)


if __name__ == "__main__":
    main()
