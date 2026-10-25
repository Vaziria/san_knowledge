@echo off
cd /d "%~dp0"
echo San Tower: open http://127.0.0.1:7477 in your browser. Close this window to stop it.
san_tower.exe serve
pause
