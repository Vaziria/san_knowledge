#!/bin/sh
# Renders both formats of every part, one after another. Logs next to each MP4.
set -u
HF="npx --yes hyperframes@0.8.165"
render() { # dir name
  ( cd "$1" && $HF render --video-frame-format png -o "renders/$2-16x9.mp4" > "renders/$2-16x9.log" 2>&1; echo "$1 16x9 exit $?" )
  ( cd "$1" && $HF render --video-frame-format png -c compositions/vertical.html -o "renders/$2-9x16.mp4" > "renders/$2-9x16.log" 2>&1; echo "$1 9x16 exit $?" )
}
render 01-fisika kubus-jatuh-rigid-body
render 02-batu batu-low-poly
render 03-mirror mirror-modifier
