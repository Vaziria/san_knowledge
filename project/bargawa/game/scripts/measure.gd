extends Node

# Measures the first walk (milestone 1): an autopilot walks Ramabargawa along the paths of Jatisrana, from the sons'
# hut past the shrine and the gate, down the lake steps to the bathing stone and back up to the pawon, with vsync off.
# It records frame times, draw calls and triangles, takes stills at five points, writes
# project/bargawa/_logs/game_measure.json and the stills into project/bargawa/renders/checks/game/, and quits.
# Run: Godot --path project/bargawa/game -- --measure

# Blender (x, y) of the route, and the stills: [x, y, still name, compass direction to look (0 north, 90 east)]
const ROUTE := [
	[5.5, 7.2, "start", 200.0], [0.0, 7.0], [-6.0, 6.8], [-10.0, 5.5], [-14.6, 8.0, "sanggar", 270.0],
	[-10.5, 2.5], [-11.0, -1.6], [-9.0, -1.2], [-5.6, -0.8], [-3.0, -6.4], [1.0, -9.0], [1.5, -12.8, "steps", 180.0],
	[2.2, -22.4], [2.6, -23.6, "lake", 225.0], [2.2, -22.4], [1.5, -12.8], [6.0, -6.0], [8.8, 4.6, "pawon", 330.0],
	[5.5, 7.2],
]
const WARMUP := 3.0         # seconds left out of the numbers (shader compiles on the first frames)
const LIMIT := 300.0

var _main: Node
var _player: Player
var _i := 0
var _t := 0.0
var _frames: PackedFloat32Array = []
var _draws: PackedInt32Array = []
var _prims: PackedInt32Array = []
var _first_seconds: PackedFloat32Array = []
var _stuck: Array = []
var _shots: Array = []
var _last_pos := Vector3.ZERO
var _last_move_t := 0.0
var _busy := false

func start(main: Node) -> void:
	_main = main
	_player = main.player
	DisplayServer.window_set_vsync_mode(DisplayServer.VSYNC_DISABLED)
	Engine.max_fps = 0
	get_viewport().get_window().title = "Ramabargawa: measuring the first walk"

static func _godot(p: Array) -> Vector3:
	return Vector3(p[0], 0.0, -p[1])

func _process(delta: float) -> void:
	_t += delta
	if _t < WARMUP:
		_first_seconds.append(delta)
	elif not _busy:                       # not while a still is taken: saving the PNG stalls that frame

		_frames.append(delta)
		_draws.append(int(Performance.get_monitor(Performance.RENDER_TOTAL_DRAW_CALLS_IN_FRAME)))
		_prims.append(int(Performance.get_monitor(Performance.RENDER_TOTAL_PRIMITIVES_IN_FRAME)))
	if _t > LIMIT:
		_finish("time limit")

func _physics_process(delta: float) -> void:
	if _busy or _player == null:
		return
	if _i >= ROUTE.size():
		_finish("route done")
		return
	var w: Array = ROUTE[_i]
	var to := _godot(w) - _player.global_position
	to.y = 0.0
	if to.length() < 0.6:
		_arrived(w)
		return
	var dir := to.normalized()
	_player.auto_dir = dir
	_player.cam_yaw = lerp_angle(_player.cam_yaw, atan2(dir.x, dir.z) + PI, clampf(3.0 * delta, 0.0, 1.0))
	if _player.global_position.distance_to(_last_pos) > 0.3:
		_last_pos = _player.global_position
		_last_move_t = _t
	elif _t - _last_move_t > 4.0:
		_stuck.append({"waypoint": _i, "at": [snappedf(_player.global_position.x, 0.1), snappedf(-_player.global_position.z, 0.1)]})
		_last_move_t = _t
		_i += 1

func _arrived(w: Array) -> void:
	_player.auto_dir = Vector3.ZERO
	_i += 1
	_last_move_t = _t
	if w.size() > 2:
		_busy = true
		_shoot(w[2], w[3])

func _shoot(shot: String, compass: float) -> void:
	# Look towards a compass direction (Blender: 0 north = Godot -Z, 90 east = +X), let it settle, take a still.
	var a := deg_to_rad(compass)
	_player.face(Vector3(sin(a), 0.0, -cos(a)))
	_player.cam_pitch = -0.12
	await get_tree().create_timer(1.2).timeout
	await RenderingServer.frame_post_draw
	var dir := ProjectSettings.globalize_path("res://").path_join("../renders/checks/game")
	DirAccess.make_dir_recursive_absolute(dir)
	var path := dir.path_join(shot + ".png")
	get_viewport().get_texture().get_image().save_png(path)
	_shots.append({"name": shot, "file": path.simplify_path(),
		"draws": Performance.get_monitor(Performance.RENDER_TOTAL_DRAW_CALLS_IN_FRAME),
		"triangles": Performance.get_monitor(Performance.RENDER_TOTAL_PRIMITIVES_IN_FRAME)})
	_player.cam_pitch = -0.22
	await get_tree().process_frame               # the frame after the save still carries its stall
	_busy = false

func _finish(why: String) -> void:
	set_process(false)
	set_physics_process(false)
	var times := _frames.duplicate()
	times.sort()
	var n := times.size()
	var total := 0.0
	for f in times:
		total += f
	var report := {
		"why": why,
		"seconds": snappedf(_t, 0.1),
		"adapter": RenderingServer.get_video_adapter_name(),
		"renderer": ProjectSettings.get_setting("rendering/renderer/rendering_method"),
		"window": [get_viewport().get_visible_rect().size.x, get_viewport().get_visible_rect().size.y],
		"scale_3d": ProjectSettings.get_setting("rendering/scaling_3d/scale"),
		"frames": n,
		"fps_average": snappedf(n / maxf(total, 0.001), 0.1),
		"fps_1pct_low": snappedf(1.0 / maxf(times[int(n * 0.99)], 0.0001), 0.1) if n else 0,
		"fps_worst": snappedf(1.0 / maxf(times[n - 1], 0.0001), 0.1) if n else 0,
		"warmup_worst_ms": snappedf(_max(_first_seconds) * 1000.0, 0.1),
		"draws_average": _avg(_draws), "draws_max": _maxi(_draws),
		"triangles_average": _avg(_prims), "triangles_max": _maxi(_prims),
		"video_memory_mb": snappedf(Performance.get_monitor(Performance.RENDER_VIDEO_MEM_USED) / 1048576.0, 0.1),
		"static_memory_mb": snappedf(Performance.get_monitor(Performance.MEMORY_STATIC) / 1048576.0, 0.1),
		"stuck": _stuck,
		"shots": _shots,
	}
	var path := ProjectSettings.globalize_path("res://").path_join("../_logs/game_measure.json").simplify_path()
	DirAccess.make_dir_recursive_absolute(path.get_base_dir())
	var f := FileAccess.open(path, FileAccess.WRITE)
	f.store_string(JSON.stringify(report, "  "))
	f.close()
	print("MEASURE ", JSON.stringify(report))
	get_tree().quit()

static func _avg(a) -> int:
	if a.is_empty():
		return 0
	var s := 0
	for v in a:
		s += v
	return s / a.size()

static func _maxi(a) -> int:
	var m := 0
	for v in a:
		m = maxi(m, v)
	return m

static func _max(a) -> float:
	var m := 0.0
	for v in a:
		m = maxf(m, v)
	return m
