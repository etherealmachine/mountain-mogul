# Mountain Mogul — model build pipeline.
#
# OpenSCAD source files in models-src/ compile to OBJ files in
# assets/models/, with an intermediate 3MF parked under build/3mf/. The Go
# converter under tools/scad2obj/ handles the SCAD-Z-up → game-Y-up
# coordinate swap and translates 3MF basematerials into per-vertex OBJ
# colours (the Wavefront `v x y z r g b` extension our loader reads).
#
# Requires the OpenSCAD development snapshot (>= 2026.04 or so) — the
# 2021.01 stable release drops color() information on export.
#
# Targets:
#   make models       — rebuild every OBJ that's stale w.r.t. its .scad
#   make clean-models — remove the intermediate 3MF cache (does NOT delete
#                       .obj files in assets/models, since some are
#                       hand-authored)

OPENSCAD  ?= openscad

SCAD_FILES := $(wildcard models-src/*.scad)
TMF_FILES  := $(SCAD_FILES:models-src/%.scad=build/3mf/%.3mf)
OBJ_FILES  := $(SCAD_FILES:models-src/%.scad=assets/models/%.obj)

.PHONY: models clean-models

models: $(OBJ_FILES)

build/3mf:
	mkdir -p $@

# OpenSCAD writes ECHO() output to stderr; we tee it into a sidecar .echo
# file so scad2obj can pick up `MOGUL_META …` lines and bake them into the
# OBJ as slot metadata. The redirection still keeps build-time errors and
# warnings visible because make prints stderr from the spawned shell on
# failure.
build/3mf/%.3mf: models-src/%.scad | build/3mf
	$(OPENSCAD) -o $@ $< 2> $(@:.3mf=.echo)

assets/models/%.obj: build/3mf/%.3mf tools/scad2obj/main.go
	go run ./tools/scad2obj $< $(<:.3mf=.echo) $@

clean-models:
	rm -rf build/3mf
