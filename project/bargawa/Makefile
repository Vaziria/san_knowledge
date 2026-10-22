# Bargawa game shortcuts. From project/bargawa: `make play`; from the repo root: `make -C project/bargawa play`.
#
#   make play         play the game (scene 1 so far): WASD to walk, click then mouse to look, Esc frees the mouse, F3 numbers
#   make export       the game's copy of the place from the built Blender file, then Godot imports it
#   make import       headless Godot import (after clean-cache, or files changed by hand)
#   make check        import, then a short headless run; prints errors only
#   make measure      walk the set route, then _logs/game_measure.json and renders/checks/game/*.png
#   make editor       open the Godot editor
#   make clean-cache  delete Godot's game/.godot cache (safe: the next import rebuilds it)
#
# play, check and measure export the place first when its export is missing or older than the Blender place, its export
# script or common/game.py. Flag: PLACE=jatisrana.
# Every target runs one job of scripts/run.ps1 (PowerShell 7), so it works the same from PowerShell, cmd or Git Bash.

PWSH ?= pwsh
RUN := $(PWSH) -NoProfile -ExecutionPolicy Bypass -File scripts/run.ps1
PLACE ?= jatisrana
EXPORT := game/assets/export/$(PLACE)/$(PLACE).glb
EXPORT_FROM := $(wildcard blender/places/$(PLACE).blend) scripts/game/$(PLACE).py scripts/common/game.py

.PHONY: help play export import check measure editor clean-cache

help:
	@echo Targets: play export import check measure editor clean-cache. Flag: PLACE=jatisrana. See the Makefile.

play: $(EXPORT)
	$(RUN) play

$(EXPORT): $(EXPORT_FROM)
	$(RUN) game $(PLACE)

export:
	$(RUN) game $(PLACE)

import:
	$(RUN) import

check: $(EXPORT)
	$(RUN) check

measure: $(EXPORT)
	$(RUN) measure

editor:
	$(RUN) editor

clean-cache:
	$(RUN) clean-cache
