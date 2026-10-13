#!/bin/sh
# ffmpeg from Debian's packages, unpacked into root/ without installing them
# (no sudo); deploy.ps1 puts it in ~/san_radio/ffmpeg/ffmpeg. Installing the
# ffmpeg package makes this unnecessary.
R="$(dirname "$(readlink -f "$0")")/root"
for L in "$R"/usr/lib/*-linux-gnu*; do :; done
export LD_LIBRARY_PATH="$L:$L/blas:$L/lapack:$L/pulseaudio${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export FONTCONFIG_FILE="$R/etc/fonts/fonts.conf"
exec "$R/usr/bin/ffmpeg" "$@"
