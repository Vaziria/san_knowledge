#!/bin/sh
# Runs San Tower on the Raspberry Pi (the user's, 2026-10-26), and again
# whenever it exits. Cron starts it at boot (@reboot), so it needs no sudo,
# as san_radio's run.sh. Only one copy runs at a time. It listens on the
# network (--lan; the tower has no login, so anyone on the LAN can use it).
# `san_tower update` restarts it in place, in the same process, so this loop
# only sees a crash or a stop. ffmpeg (Cut Video, Crop Video) is san_radio's
# unpacked one when none is installed. The log is san_tower.log, kept under
# 5 MB (the older part in san_tower.log.old).
cd "$(dirname "$(readlink -f "$0")")" || exit 1

exec 9>.run.lock
if ! flock -n 9; then
	echo "san_tower is running already" >&2
	exit 0
fi

if ! command -v ffmpeg >/dev/null && [ -x "$HOME/san_radio/ffmpeg/ffmpeg" ]; then
	PATH="$HOME/san_radio/ffmpeg:$PATH"
	export PATH
fi

while true; do
	if [ "$(stat -c %s san_tower.log 2>/dev/null || echo 0)" -gt 5000000 ]; then
		mv san_tower.log san_tower.log.old
	fi
	./san_tower serve --listen 0.0.0.0:7477 --lan --data data >>san_tower.log 2>&1
	sleep 10
done
