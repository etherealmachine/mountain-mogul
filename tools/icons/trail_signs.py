"""Draws assets/icons/trail-signs.png: the trail difficulty signs, a green
circle, a blue square and a black diamond, each outlined in white, for the
Trail tool's button.

    python3 tools/icons/trail_signs.py
"""
from PIL import Image, ImageDraw

S = 1024  # drawn 4x and scaled down, for smooth edges
OUT = 6 * 4  # white outline
img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
d = ImageDraw.Draw(img)

green = (52, 168, 83, 255)
blue = (38, 110, 220, 255)
black = (20, 20, 22, 255)
white = (255, 255, 255, 255)


def circle(cx, cy, r, fill):
    d.ellipse([cx - r - OUT, cy - r - OUT, cx + r + OUT, cy + r + OUT], fill=white)
    d.ellipse([cx - r, cy - r, cx + r, cy + r], fill=fill)


def square(cx, cy, h, fill):
    d.rectangle([cx - h - OUT, cy - h - OUT, cx + h + OUT, cy + h + OUT], fill=white)
    d.rectangle([cx - h, cy - h, cx + h, cy + h], fill=fill)


def diamond(cx, cy, r, fill):
    o = OUT * 1.42
    d.polygon([(cx, cy - r - o), (cx + r + o, cy), (cx, cy + r + o), (cx - r - o, cy)], fill=white)
    d.polygon([(cx, cy - r), (cx + r, cy), (cx, cy + r), (cx - r, cy)], fill=fill)


# Two up top, the diamond under them.
circle(300, 310, 190, green)
square(724, 310, 170, blue)
diamond(512, 735, 250, black)

img.resize((256, 256), Image.LANCZOS).save("assets/icons/trail-signs.png")
