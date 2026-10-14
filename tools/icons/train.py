"""Draws assets/icons/train.png: a train's front in white, in the style
of the Phosphor icons the toolbar uses, for the Rail tools' buttons.

    python3 tools/icons/train.py
"""
from PIL import Image, ImageDraw

S = 1024  # drawn 4x and scaled down, for smooth edges
W = 56    # stroke
img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
d = ImageDraw.Draw(img)
white = (255, 255, 255, 255)
# The body: a rounded front with a stroke.
d.rounded_rectangle([272, 128, 752, 768], radius=110, outline=white, width=W)
# The windscreen.
d.rounded_rectangle([340, 220, 684, 470], radius=40, fill=white)
# Two lamps.
d.ellipse([350, 560, 450, 660], fill=white)
d.ellipse([574, 560, 674, 660], fill=white)
# The rails under it, splaying out.
d.line([(380, 768), (300, 920)], fill=white, width=W)
d.line([(644, 768), (724, 920)], fill=white, width=W)
img.resize((256, 256), Image.LANCZOS).save("assets/icons/train.png")
