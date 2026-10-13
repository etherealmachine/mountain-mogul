# Mountain Mogul — model build pipeline.
#
# Each model is a Python script in models-src/ that builds its parts with
# build123d and writes its OBJ to assets/models/ (models-src/lib/mogul.py:
# the helpers, colours, metadata and the OBJ writer). The scripts run in a
# project virtualenv, .venv-models, made by `make models-venv`.
#
# Targets:
#   make models-venv  — create .venv-models with models-src/requirements.txt
#   make models       — rebuild every OBJ that's stale w.r.t. its script or lib/
#   make models -j8   — the same, in parallel

PYTHON    ?= .venv-models/bin/python

MODEL_SRCS := $(wildcard models-src/*.py)
OBJ_FILES  := $(MODEL_SRCS:models-src/%.py=assets/models/%.obj)
MODEL_LIB  := $(wildcard models-src/lib/*.py)

.PHONY: models models-venv

models: $(OBJ_FILES)

models-venv:
	python3 -m venv .venv-models
	.venv-models/bin/pip install -r models-src/requirements.txt

assets/models/%.obj: models-src/%.py $(MODEL_LIB)
	$(PYTHON) $< $@

DEMO_DIR    := build/demo
DEMO_BIN    := $(DEMO_DIR)/mountain-mogul-demo
# Models of things the demo leaves out.
DEMO_NO_MODELS := bar.obj chair_6pack.obj lift_terminal_wide.obj gondola_cabin.obj helicopter_% helipad.obj snow_gun.obj tent_%

.PHONY: demo demo-audit

demo:
	rm -rf $(DEMO_DIR)
	mkdir -p $(DEMO_DIR)/assets/scenarios $(DEMO_DIR)/assets/models
	go build -tags demo -trimpath -ldflags "-s -w" -o $(DEMO_BIN) .
	cp -R assets/fonts assets/icons assets/shaders assets/logo.png $(DEMO_DIR)/assets/
	cp assets/scenarios/boreal.save $(DEMO_DIR)/assets/scenarios/
	cp $(filter-out $(addprefix assets/models/,$(DEMO_NO_MODELS)),$(wildcard assets/models/*.obj)) $(DEMO_DIR)/assets/models/

demo-audit: demo
	tools/demo-audit.sh $(DEMO_BIN)
