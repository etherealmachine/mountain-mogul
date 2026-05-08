# Mountain Mogul — model build pipeline.
#
# OpenSCAD source files in models-src/ compile to OBJ files in
# assets/models/, with an intermediate ASCII STL parked under build/stl/.
# The Go converter under tools/stl2obj/ handles the SCAD-Z-up → game-Y-up
# coordinate swap.
#
# Targets:
#   make models       — rebuild every OBJ that's stale w.r.t. its .scad
#   make clean-models — remove the intermediate STL cache (does NOT delete
#                       .obj files in assets/models, since some are
#                       hand-authored)

OPENSCAD  ?= openscad

SCAD_FILES := $(wildcard models-src/*.scad)
STL_FILES  := $(SCAD_FILES:models-src/%.scad=build/stl/%.stl)
OBJ_FILES  := $(SCAD_FILES:models-src/%.scad=assets/models/%.obj)

.PHONY: models clean-models

models: $(OBJ_FILES)

build/stl:
	mkdir -p $@

build/stl/%.stl: models-src/%.scad | build/stl
	$(OPENSCAD) -o $@ $<

assets/models/%.obj: build/stl/%.stl tools/stl2obj/main.go
	go run ./tools/stl2obj $< $@

clean-models:
	rm -rf build/stl
