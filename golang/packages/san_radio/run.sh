#!/bin/sh
# Runs san_radio on the Raspberry Pi, and again whenever it exits. Cron starts
# it at boot (@reboot), so it needs no sudo: no systemd service, and ffmpeg
# unpacked into ffmpeg/ (see ffmpeg/ffmpeg) when it isn't installed. Only one
# copy runs at a time: one stream key carries one stream. The log is
# san_radio.log, kept under 5 MB (the older part in san_radio.log.old).
cd "$(dirname "$(readlink -f "$0")")" || exit 1

exec 9>.run.lock
if ! flock -n 9; then
	echo "san_radio is running already" >&2
	exit 0
fi

ffmpeg=ffmpeg
if ! command -v ffmpeg >/dev/null; then
	ffmpeg=ffmpeg/ffmpeg
fi

# 24 fps: on a Pi 3B+ held at 600 MHz by a weak power supply, the three
# panes fell behind at 30 (0.92x).
while true; do
	if [ "$(stat -c %s san_radio.log 2>/dev/null || echo 0)" -gt 5000000 ]; then
		mv san_radio.log san_radio.log.old
	fi
	./san_radio -ffmpeg "$ffmpeg" -fps 24 -config channels.yaml -video loop.mp4 -music music >>san_radio.log 2>&1
	sleep 10
done
