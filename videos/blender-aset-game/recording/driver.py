"""Drives the tutorial Blender like a person (real cursor moves, clicks and key presses) and
records the screen with ffmpeg. Every input is logged with its time in the video, so the edit
can add key labels, a cursor highlight, click ripples and zooms that match exactly.

    python driver.py launch <part> [--ui-scale 1.25]  start the tutorial Blender for a part (fisika, batu, mirror)
    python driver.py exec "<python>"            run Python in it (helpers: layout, project, frame_view)
    python driver.py layout                     area/region rects in screen pixels
    python driver.py shot <png> [x y w h]       screenshot, optionally cropped
    python driver.py move X Y | click X Y | dclick X Y | key NAME   one input, for calibration
    python driver.py take <part> <name>         fresh Blender, then record steps_<part>.py into out/<part>-<name>.mp4
    python driver.py quit                       close the tutorial Blender

Safety: inputs go only to the tutorial Blender. Before every click or key the driver checks
that Blender is the foreground window and that nobody moved the mouse; otherwise it stops.
"""
import ctypes
import ctypes.wintypes as wt
import importlib.util
import json
import math
import os
import random
import re
import subprocess
import sys
import threading
import time
import uuid

HERE = os.path.dirname(os.path.abspath(__file__))
STATE = os.path.join(HERE, ".state")
CHANNEL = os.path.join(STATE, "channel")
OUT_DIR = os.path.join(HERE, "out")
BLENDER = os.environ.get("TUTORIAL_BLENDER", r"D:\blender\blender.exe")

ctypes.windll.shcore.SetProcessDpiAwareness(2)
user32 = ctypes.windll.user32
kernel32 = ctypes.windll.kernel32


class KEYBDINPUT(ctypes.Structure):
    _fields_ = [("wVk", wt.WORD), ("wScan", wt.WORD), ("dwFlags", wt.DWORD),
                ("time", wt.DWORD), ("dwExtraInfo", ctypes.c_size_t)]


class MOUSEINPUT(ctypes.Structure):
    _fields_ = [("dx", wt.LONG), ("dy", wt.LONG), ("mouseData", wt.DWORD), ("dwFlags", wt.DWORD),
                ("time", wt.DWORD), ("dwExtraInfo", ctypes.c_size_t)]


class _INPUTUNION(ctypes.Union):
    _fields_ = [("ki", KEYBDINPUT), ("mi", MOUSEINPUT)]


class INPUT(ctypes.Structure):
    _fields_ = [("type", wt.DWORD), ("u", _INPUTUNION)]


INPUT_MOUSE, INPUT_KEYBOARD = 0, 1
MOUSE_LEFTDOWN, MOUSE_LEFTUP = 0x0002, 0x0004
MOUSE_WHEEL = 0x0800
KEYUP, EXTENDED = 0x0002, 0x0001

VK = {"Enter": 0x0D, "Esc": 0x1B, "Tab": 0x09, "Space": 0x20, "Backspace": 0x08,
      "Ctrl": 0xA2, "Shift": 0xA0, "Alt": 0xA4, "Delete": 0x2E, ".": 0xBE, "-": 0xBD,
      "Home": 0x24, "F12": 0x7B, "Left": 0x25, "Right": 0x27}
VK.update({chr(c): c for c in range(ord("A"), ord("Z") + 1)})
VK.update({str(d): 0x30 + d for d in range(10)})
EXTENDED_KEYS = {"Delete", "Home", "Left", "Right"}


class Abort(Exception):
    pass


def _send(*inputs):
    arr = (INPUT * len(inputs))(*inputs)
    if user32.SendInput(len(inputs), arr, ctypes.sizeof(INPUT)) != len(inputs):
        raise Abort("SendInput was blocked")


def _key_input(name, up):
    vk = VK[name]
    flags = (KEYUP if up else 0) | (EXTENDED if name in EXTENDED_KEYS else 0)
    return INPUT(INPUT_KEYBOARD, _INPUTUNION(ki=KEYBDINPUT(vk, user32.MapVirtualKeyW(vk, 0), flags, 0, 0)))


def _mouse_input(flags, data=0):
    return INPUT(INPUT_MOUSE, _INPUTUNION(mi=MOUSEINPUT(0, 0, data, flags, 0, 0)))


def _rel_move(dx, dy):
    return INPUT(INPUT_MOUSE, _INPUTUNION(mi=MOUSEINPUT(dx, dy, 0, 0x0001, 0, 0)))


def cursor_pos():
    p = wt.POINT()
    user32.GetCursorPos(ctypes.byref(p))
    return p.x, p.y


def window_pid(hwnd):
    pid = wt.DWORD()
    user32.GetWindowThreadProcessId(hwnd, ctypes.byref(pid))
    return pid.value


def main_window(pid):
    found = []

    @ctypes.WINFUNCTYPE(wt.BOOL, wt.HWND, wt.LPARAM)
    def cb(hwnd, _):
        if window_pid(hwnd) == pid and user32.IsWindowVisible(hwnd):
            found.append(hwnd)
        return True

    user32.EnumWindows(cb, 0)
    return found[0] if found else None


def bring_to_front(hwnd):
    fg = user32.GetForegroundWindow()
    fg_thread = user32.GetWindowThreadProcessId(fg, None)
    me = kernel32.GetCurrentThreadId()
    user32.AttachThreadInput(me, fg_thread, True)
    user32.ShowWindow(hwnd, 5)
    user32.BringWindowToTop(hwnd)
    user32.SetForegroundWindow(hwnd)
    user32.AttachThreadInput(me, fg_thread, False)
    time.sleep(0.3)
    return user32.GetForegroundWindow() == hwnd


# ---------------------------------------------------------------- tutorial Blender

def load_state():
    try:
        with open(os.path.join(STATE, "state.json"), encoding="utf-8") as f:
            return json.load(f)
    except FileNotFoundError:
        return {}


def blender_exec(code, timeout=30):
    os.makedirs(os.path.join(CHANNEL, "in"), exist_ok=True)
    cid = uuid.uuid4().hex[:12]
    tmp = os.path.join(CHANNEL, "in", cid + ".tmp")
    with open(tmp, "w", encoding="utf-8") as f:
        f.write(code)
    os.replace(tmp, os.path.join(CHANNEL, "in", cid + ".py"))
    out = os.path.join(CHANNEL, "out", cid + ".json")
    end = time.time() + timeout
    while time.time() < end:
        if os.path.exists(out):
            try:
                with open(out, encoding="utf-8") as f:
                    res = json.load(f)
                os.remove(out)
            except (PermissionError, json.JSONDecodeError):
                # Windows Defender scans new files and briefly locks them.
                time.sleep(0.01)
                continue
            if not res["ok"]:
                raise RuntimeError(res["error"])
            return res
        time.sleep(0.01)
    raise TimeoutError("tutorial Blender did not answer")


class ValueWatch(threading.Thread):
    """Reads a value from the tutorial Blender as fast as the channel answers (~20-40 per second)."""

    def __init__(self, expr):
        super().__init__(daemon=True)
        self.code = "result = " + expr
        self.v = self.t = None
        self.history = []
        self.stop = False

    def run(self):
        while not self.stop:
            sent = time.time()
            try:
                v = blender_exec(self.code, timeout=5)["result"]
            except (OSError, TimeoutError):
                continue
            self.t = (sent + time.time()) / 2
            self.v = v
            self.history.append((self.t, v))


def quit_blender():
    pid = load_state().get("pid")
    if pid:
        subprocess.run(["taskkill", "/PID", str(pid), "/F"], capture_output=True)
        time.sleep(1.0)


def launch(part, ui_scale=1.25):
    quit_blender()
    for sub in ("in", "out"):
        d = os.path.join(CHANNEL, sub)
        os.makedirs(d, exist_ok=True)
        for name in os.listdir(d):
            os.remove(os.path.join(d, name))
    ready = os.path.join(CHANNEL, "ready.json")
    if os.path.exists(ready):
        os.remove(ready)
    log = open(os.path.join(STATE, "blender.log"), "w", encoding="utf-8")
    flags = subprocess.DETACHED_PROCESS | subprocess.CREATE_NEW_PROCESS_GROUP
    p = subprocess.Popen([BLENDER, "--factory-startup", "--python", os.path.join(HERE, "blender_session.py"),
                          "--", CHANNEL, str(ui_scale), part], stdout=log, stderr=subprocess.STDOUT,
                         creationflags=flags, cwd=STATE)
    with open(os.path.join(STATE, "state.json"), "w", encoding="utf-8") as f:
        json.dump({"pid": p.pid, "ui_scale": ui_scale, "part": part}, f)
    end = time.time() + 90
    while not os.path.exists(ready):
        if time.time() > end or p.poll() is not None:
            raise RuntimeError("tutorial Blender did not start; see .state/blender.log")
        time.sleep(0.2)
    time.sleep(1.5)
    hwnd = main_window(p.pid)
    if not hwnd or not bring_to_front(hwnd):
        raise RuntimeError("could not bring the tutorial Blender to the front")
    return p.pid


# ---------------------------------------------------------------- inputs + log

class Driver:
    """Human-paced input to the tutorial Blender, logged against the wall clock."""

    def __init__(self, seed=7):
        self.pid = load_state().get("pid")
        self.rng = random.Random(seed)
        self.events = []
        self.last_set = None
        self._last_pos_log = 0.0

    def log(self, kind, **kw):
        self.events.append(dict(wall=time.time(), type=kind, **kw))

    def check(self):
        fg = user32.GetForegroundWindow()
        if window_pid(fg) != self.pid:
            raise Abort("Blender is not the foreground window")
        if self.last_set is not None:
            x, y = cursor_pos()
            if abs(x - self.last_set[0]) > 3 or abs(y - self.last_set[1]) > 3:
                raise Abort("the mouse was moved by someone else")

    def _set(self, x, y, force_log=False):
        user32.SetCursorPos(int(round(x)), int(round(y)))
        self.last_set = (int(round(x)), int(round(y)))
        now = time.time()
        if force_log or now - self._last_pos_log >= 1 / 120:
            self.log("pos", x=self.last_set[0], y=self.last_set[1])
            self._last_pos_log = now

    def move(self, x, y, dur=None):
        self.check()
        x0, y0 = cursor_pos()
        dist = math.hypot(x - x0, y - y0)
        if dist < 1:
            return
        dur = dur or min(1.1, 0.4 + dist / 1500)
        nx, ny = -(y - y0) / dist, (x - x0) / dist
        bend = self.rng.uniform(-0.07, 0.07) * dist
        t0 = time.perf_counter()
        while True:
            u = min(1.0, (time.perf_counter() - t0) / dur)
            s = u * u * u * (10 - 15 * u + 6 * u * u)
            arc = math.sin(math.pi * s) * bend
            self._set(x0 + (x - x0) * s + nx * arc, y0 + (y - y0) * s + ny * arc, force_log=u >= 1)
            if u >= 1:
                break
            time.sleep(1 / 240)

    def click(self, x=None, y=None, label=None, settle=0.18):
        if x is not None:
            self.move(x, y)
            time.sleep(settle)
        self.check()
        cx, cy = cursor_pos()
        _send(_mouse_input(MOUSE_LEFTDOWN))
        self.log("down", x=cx, y=cy)
        time.sleep(self.rng.uniform(0.06, 0.09))
        _send(_mouse_input(MOUSE_LEFTUP))
        self.log("up", x=cx, y=cy)
        self.log("click", x=cx, y=cy, label=label)

    def dclick(self, x=None, y=None, label=None):
        self.click(x, y, label=label)
        time.sleep(0.09)
        self.click(label=None)

    def scrub(self, x0, y0, value_expr, moves, slider, label=None):
        """Press on a slider at (x0, y0) and drag it through `moves` = [(target value, seconds,
        hold after)], then release.

        While a slider drags, Blender hides the cursor, wraps it around the window edges and reads
        raw mouse motion (SetCursorPos is ignored), and Windows pointer acceleration scales that
        motion. So this sends relative moves and steers by the slider's real value, read live from
        Blender (`value_expr`), with a feed-forward rate learned on the way. The logged cursor
        follows the slider's fill edge (`slider` = (left x, width)), where the hidden cursor acts."""
        self.move(x0, y0)
        time.sleep(0.25)
        self.check()
        watch = ValueWatch(value_expr)
        watch.start()
        while watch.v is None:
            time.sleep(0.02)
        _send(_mouse_input(MOUSE_LEFTDOWN))
        self.log("down", x=x0, y=y0)
        self.log("drag", x=x0, y=y0, label=label)
        time.sleep(0.15)
        left, width = slider
        vpm = 0.004            # slider value per mickey, refined from what Blender reports
        acc = 0.0
        sent = [(time.time(), 0)]  # cumulative mickeys over time
        total = 0
        plan = []              # (wall time, desired value)
        last_seen = watch.t
        try:
            for target, dur, hold in moves:
                v0 = watch.v
                t0 = time.time()
                prev_des = v0
                while True:
                    now = time.time()
                    if now - watch.t > 0.4:
                        # No fresh reading: don't drive blind. Stop the drag if it lasts.
                        if now - watch.t > 3.0:
                            raise Abort("lost the slider value from Blender")
                        t0 += 1 / 200
                        time.sleep(1 / 200)
                        continue
                    u = min(1.0, (now - t0) / dur)
                    des = v0 + (target - v0) * (0.5 - 0.5 * math.cos(math.pi * u))
                    plan.append((now, des))
                    # Compare the latest reading with what was wanted when it was taken.
                    want_then = next((d for t, d in reversed(plan) if t <= watch.t), des)
                    acc += (des - prev_des) / vpm + 4.0 * (want_then - watch.v) / vpm / 200
                    prev_des = des
                    m = int(acc)
                    if m:
                        m = max(-6, min(6, m))
                        _send(_rel_move(m, 0))
                        acc -= m
                        total += m
                        sent.append((now, total))
                    if watch.t != last_seen:
                        last_seen = watch.t
                        self.log("pos", x=round(left + max(0.0, min(1.0, watch.v)) * width), y=y0)
                        # Refine vpm from the last ~0.5 s of motion.
                        past = next(((t, s) for t, s in sent if t >= watch.t - 0.5), None)
                        cur = next((s for t, s in reversed(sent) if t <= watch.t), None)
                        hist = next((v for t, v in reversed(watch.history) if t <= (past or (0, 0))[0]), None)
                        if past and cur is not None and hist is not None and abs(cur - past[1]) >= 15:
                            est = (watch.v - hist) / (cur - past[1])
                            if 0.0015 < est < 0.02:
                                vpm = 0.7 * vpm + 0.3 * est
                    if u >= 1 and (abs(target - watch.v) < 0.015 or now - t0 > dur + 1.0):
                        break
                    time.sleep(1 / 200)
                time.sleep(hold)
        finally:
            _send(_mouse_input(MOUSE_LEFTUP))
            watch.stop = True
        time.sleep(0.2)
        # Show the cursor again where the slider stopped, not wherever the wrap left it.
        self._set(left + watch.v * width, y0, force_log=True)
        self.log("up", x=self.last_set[0], y=y0)
        self.log("value", value=round(watch.v, 3))

    def key(self, combo, label=None, show=True):
        """combo like "Z", "Enter", "Ctrl+Space"."""
        self.check()
        names = combo.split("+")
        for n in names:
            _send(_key_input(n, False))
            time.sleep(0.03)
        time.sleep(self.rng.uniform(0.05, 0.08))
        for n in reversed(names):
            _send(_key_input(n, True))
            time.sleep(0.02)
        if show:
            self.log("key", keys=names, label=label or " + ".join(names))

    def type_text(self, text, label=None):
        self.log("type", keys=[text], label=label or text)
        for ch in text:
            self.key(ch.upper() if ch.isalpha() else ch, show=False)
            time.sleep(self.rng.uniform(0.09, 0.16))

    def wheel(self, notches):
        self.check()
        for _ in range(abs(notches)):
            _send(_mouse_input(MOUSE_WHEEL, (120 if notches > 0 else -120) & 0xFFFFFFFF))
            time.sleep(0.05)
        self.log("wheel", notches=notches)

    def step(self, sid, caption):
        self.log("step", id=sid, caption=caption)

    def hold(self, sec):
        end = time.time() + sec
        while time.time() < end:
            self.check()
            time.sleep(0.05)


# ---------------------------------------------------------------- recording

class Recorder:
    def __init__(self, path, fps=30):
        self.path = path
        self.fps = fps
        self.start_wall = None
        self.stderr = []

    def start(self):
        cmd = ["ffmpeg", "-hide_banner", "-f", "gdigrab", "-framerate", str(self.fps), "-draw_mouse", "1",
               "-i", "desktop", "-c:v", "libx264", "-preset", "ultrafast", "-crf", "14",
               "-pix_fmt", "yuv420p", "-g", str(self.fps * 2), "-y", self.path]
        self.p = subprocess.Popen(cmd, stdin=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
                                  creationflags=subprocess.CREATE_NO_WINDOW)
        threading.Thread(target=self._read, daemon=True).start()
        end = time.time() + 15
        while self.start_wall is None:
            if time.time() > end or self.p.poll() is not None:
                raise RuntimeError("ffmpeg did not start:\n" + "".join(self.stderr[-20:]))
            time.sleep(0.02)

    def _read(self):
        for line in self.p.stderr:
            self.stderr.append(line)
            m = re.search(r"start: (\d+\.\d+)", line)
            if m and self.start_wall is None:
                self.start_wall = float(m.group(1))

    def stop(self):
        try:
            self.p.stdin.write("q")
            self.p.stdin.flush()
        except OSError:
            pass
        self.p.wait(60)


def shot(png, crop=None):
    vf = ["-vf", "crop={2}:{3}:{0}:{1}".format(*crop)] if crop else []
    subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "gdigrab", "-i", "desktop",
                    "-frames:v", "1", *vf, "-y", png], check=True)


def load_steps(part):
    spec = importlib.util.spec_from_file_location("steps_" + part, os.path.join(HERE, "steps_%s.py" % part))
    steps = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(steps)
    return steps


def prepare_take(part, steps):
    """Fresh tutorial Blender for the part, with its slow first-time things done off camera
    (`steps.PREWARM`: (python, seconds to wait) pairs, ending back at the start state)."""
    launch(part, load_state().get("ui_scale", 1.25))
    for code, wait in getattr(steps, "PREWARM", []):
        blender_exec(code)
        time.sleep(wait)
    time.sleep(1.0)
    hwnd = main_window(load_state()["pid"])
    if not bring_to_front(hwnd):
        raise RuntimeError("could not bring the tutorial Blender to the front")


def take(part, name):
    os.makedirs(OUT_DIR, exist_ok=True)
    steps = load_steps(part)
    steps.blender_exec = blender_exec  # steps project 3D points to screen pixels at run time
    name = part + "-" + name
    prepare_take(part, steps)
    d = Driver()
    park = steps.PARK
    user32.SetCursorPos(*park)
    d.last_set = tuple(park)
    video = os.path.join(OUT_DIR, name + ".mp4")
    rec = Recorder(video)
    rec.start()
    status = "ok"
    try:
        d._set(*park, force_log=True)
        steps.run(d)
    except Abort as e:
        status = "aborted: " + str(e)
    finally:
        time.sleep(0.5)
        rec.stop()
    for ev in d.events:
        ev["t"] = round(ev.pop("wall") - rec.start_wall, 4)
    with open(os.path.join(OUT_DIR, name + ".events.json"), "w", encoding="utf-8") as f:
        json.dump({"video": name + ".mp4", "fps": rec.fps, "screen": [1920, 1080], "status": status,
                   "events": d.events}, f, indent=1)
    print(status, video)


def main(argv):
    os.makedirs(STATE, exist_ok=True)
    cmd, args = argv[0], argv[1:]
    if cmd == "launch":
        scale = float(args[args.index("--ui-scale") + 1]) if "--ui-scale" in args else 1.25
        print("pid", launch(args[0], scale))
    elif cmd == "quit":
        quit_blender()
    elif cmd == "exec":
        res = blender_exec(args[0])
        print(res["stdout"], end="")
        print(json.dumps(res["result"]))
    elif cmd == "layout":
        print(json.dumps(blender_exec("result = layout()")["result"], indent=1))
    elif cmd == "shot":
        shot(args[0], [int(v) for v in args[1:5]] if len(args) >= 5 else None)
    elif cmd in ("move", "click", "dclick"):
        d = Driver()
        getattr(d, cmd)(int(args[0]), int(args[1]))
    elif cmd == "key":
        Driver().key(args[0])
    elif cmd == "front":
        print(bring_to_front(main_window(load_state()["pid"])))
    elif cmd == "take":
        take(args[0], args[1])
    else:
        raise SystemExit(__doc__)


if __name__ == "__main__":
    main(sys.argv[1:])
