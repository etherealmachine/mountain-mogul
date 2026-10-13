#!/bin/sh
# Rasterises the Minty Fresh logo (assets/logo.svg) to assets/logo.png at
# four times its 240x320 size, for the intro screen. Needs rsvg-convert
# (brew install librsvg).
set -e
cd "$(dirname "$0")/../.."
rsvg-convert -w 960 -h 1280 assets/logo.svg -o assets/logo.png
