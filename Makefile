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
DEMO_NAME   := Mountain Mogul Demo
DIST        := build/dist
DEMO_FLAGS  := -tags demo -trimpath
DEMO_LD     := -s -w
# Models of things the demo leaves out.
DEMO_NO_MODELS := bar.obj chair_6pack.obj lift_terminal_wide.obj gondola_cabin.obj helicopter_% helipad.obj snow_gun.obj tent_%

# demo_assets copies the demo's assets into the folder $(1).
define demo_assets
	mkdir -p "$(1)/scenarios" "$(1)/models"
	cp -R assets/fonts assets/icons assets/shaders assets/logo.png assets/menu.save "$(1)/"
	cp assets/scenarios/boreal.save "$(1)/scenarios/"
	cp $(filter-out $(addprefix assets/models/,$(DEMO_NO_MODELS)),$(wildcard assets/models/*.obj)) "$(1)/models/"
endef

.PHONY: demo demo-audit demo-mac demo-windows demo-linux demo-cross demo-all

# The demo for this machine, to run in place.
demo:
	rm -rf $(DEMO_DIR)
	go build $(DEMO_FLAGS) -ldflags "$(DEMO_LD)" -o $(DEMO_BIN) .
	$(call demo_assets,$(DEMO_DIR)/assets)

demo-audit: demo
	tools/demo-audit.sh $(DEMO_BIN)

# The release builds, each audited and zipped into build/dist. They use
# cgo (GLFW, OpenGL), so each needs a C compiler for its platform:
# demo-mac runs on a Mac (Xcode's clang builds both architectures);
# demo-windows needs MinGW (CC, x86_64-w64-mingw32-gcc by default);
# demo-linux runs on Linux with the X11 and GL headers. From a Mac,
# demo-cross builds Windows and Linux in Docker (tools/dist/Dockerfile),
# and demo-all builds all three.

# macOS: a universal (Apple silicon and Intel) app, assets in Resources,
# for macOS 11 and later.
MAC_APP := build/demo-mac/$(DEMO_NAME).app
MAC_ENV := CGO_ENABLED=1 GOOS=darwin MACOSX_DEPLOYMENT_TARGET=11.0 CGO_CFLAGS="-mmacosx-version-min=11.0" CGO_LDFLAGS="-mmacosx-version-min=11.0"
demo-mac:
	rm -rf build/demo-mac
	mkdir -p "$(MAC_APP)/Contents/MacOS" "$(MAC_APP)/Contents/Resources" $(DIST)
	$(MAC_ENV) GOARCH=arm64 go build $(DEMO_FLAGS) -ldflags "$(DEMO_LD)" -o build/demo-mac/arm64 .
	$(MAC_ENV) GOARCH=amd64 go build $(DEMO_FLAGS) -ldflags "$(DEMO_LD)" -o build/demo-mac/amd64 .
	lipo -create -output "$(MAC_APP)/Contents/MacOS/mountain-mogul-demo" build/demo-mac/arm64 build/demo-mac/amd64
	rm build/demo-mac/arm64 build/demo-mac/amd64
	cp tools/dist/Info.plist "$(MAC_APP)/Contents/"
	$(call demo_assets,$(MAC_APP)/Contents/Resources/assets)
	codesign --force --deep --sign - "$(MAC_APP)"
	tools/demo-audit.sh "$(MAC_APP)/Contents/MacOS/mountain-mogul-demo"
	rm -f $(DIST)/mountain-mogul-demo-mac.zip
	cd build/demo-mac && ditto -c -k --keepParent "$(DEMO_NAME).app" ../dist/mountain-mogul-demo-mac.zip

# Windows: 64-bit, no console window.
WIN_CC ?= x86_64-w64-mingw32-gcc
demo-windows:
	rm -rf build/demo-windows
	mkdir -p "build/demo-windows/$(DEMO_NAME)" $(DIST)
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=$(WIN_CC) go build $(DEMO_FLAGS) -ldflags "$(DEMO_LD) -H windowsgui" -o "build/demo-windows/$(DEMO_NAME)/$(DEMO_NAME).exe" .
	$(call demo_assets,build/demo-windows/$(DEMO_NAME)/assets)
	tools/demo-audit.sh "build/demo-windows/$(DEMO_NAME)/$(DEMO_NAME).exe"
	rm -f $(DIST)/mountain-mogul-demo-windows.zip
	cd build/demo-windows && zip -qr ../dist/mountain-mogul-demo-windows.zip "$(DEMO_NAME)"

# Linux: 64-bit (x86-64).
LINUX_CC ?= gcc
demo-linux:
	rm -rf build/demo-linux
	mkdir -p build/demo-linux/mountain-mogul-demo $(DIST)
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=$(LINUX_CC) go build $(DEMO_FLAGS) -ldflags "$(DEMO_LD)" -o build/demo-linux/mountain-mogul-demo/mountain-mogul-demo .
	$(call demo_assets,build/demo-linux/mountain-mogul-demo/assets)
	tools/demo-audit.sh build/demo-linux/mountain-mogul-demo/mountain-mogul-demo
	tar -C build/demo-linux -czf $(DIST)/mountain-mogul-demo-linux.tar.gz mountain-mogul-demo

# Windows and Linux from any machine with Docker: the build image, then
# both builds in it, with Go's caches kept in volumes between runs. The
# source goes in read-only (Docker VMs often mount home that way) and is
# built in a copy; the zips come back out into build/dist.
demo-cross:
	docker build -q -t mountain-mogul-build tools/dist
	mkdir -p $(DIST)
	docker run --rm --platform linux/amd64 -v "$(CURDIR)":/src:ro -v mountain-mogul-gocache:/root/.cache -v mountain-mogul-gomod:/root/go/pkg/mod \
		mountain-mogul-build sh -c 'mkdir /work && tar -C /src --exclude=./build --exclude=./.venv-models --exclude=./.git -cf - . | tar -C /work -xf - \
		&& make -C /work demo-windows demo-linux >&2 && tar -C /work/build/dist -cf - .' > $(DIST)/cross.tar
	tar -C $(DIST) -xf $(DIST)/cross.tar && rm $(DIST)/cross.tar

demo-all: demo-mac demo-cross
