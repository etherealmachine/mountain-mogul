"""Draws assets/icons/mountains.png: two peaks in white outline with a
snowcap, in the style of the Phosphor icons the toolbar uses, for the
editor's Labels tool.

    python3 tools/icons/mountains.py
"""
from PIL import Image, ImageDraw

S = 1024  # drawn 4x and scaled down, for smooth edges
W = 56    # stroke
img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
d = ImageDraw.Draw(img)
white = (255, 255, 255, 255)
# The far peak, right, and the near one, left and taller.
d.line([(470, 840), (680, 430), (900, 840)], fill=white, width=W, joint="curve")
d.line([(124, 840), (400, 240), (676, 840)], fill=white, width=W, joint="curve")
# The near peak's snowcap.
d.polygon([(400, 240), (318, 418), (360, 400), (400, 440), (440, 400), (482, 418)], fill=white)
# The ground.
d.line([(100, 840), (924, 840)], fill=white, width=W)
img.resize((256, 256), Image.LANCZOS).save("assets/icons/mountains.png")
