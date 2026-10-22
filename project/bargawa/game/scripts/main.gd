extends Node3D

# Scene 1, Esuk (S1, morning dew), as the first walk (milestone 1): Pertapan Jatisrana in the morning light, and
# Ramabargawa as a stand-in. The light states come in milestone 4 (docs/summaries/models/jatisrana.md).
# F3 shows the frame rate, draw calls and triangles. `-- --measure` walks a set route, records them and quits
# (scripts/measure.gd).

const PLACE := "res://assets/export/jatisrana/jatisrana.json"
const KEYS := {
	"move_forward": [KEY_W, KEY_UP], "move_back": [KEY_S, KEY_DOWN],
	"move_left": [KEY_A, KEY_LEFT], "move_right": [KEY_D, KEY_RIGHT],
	"interact": [KEY_E, KEY_SPACE], "debug_overlay": [KEY_F3],
}

var place: Place
var player: Player
var _overlay: Label

func _ready() -> void:
	_keys()
	place = Place.new()
	place.name = "Place"
	add_child(place)
	place.load_place(PLACE)
	_morning(place.layout)
	player = Player.new()
	player.name = "Player"
	player.place = place
	add_child(player)
	var s := place.spawn()
	player.global_position = s["position"] + Vector3.UP * 2.0
	player.face(Vector3.BACK if s["face"] == "south" else Vector3.FORWARD)
	await get_tree().physics_frame              # the place's colliders are in the physics space from the next step
	player.global_position = _ground(s["position"])
	_overlay = Label.new()
	_overlay.position = Vector2(12, 8)
	_overlay.add_theme_color_override("font_shadow_color", Color.BLACK)
	_overlay.visible = false
	var ui := CanvasLayer.new()
	ui.add_child(_overlay)
	add_child(ui)
	if "--measure" in OS.get_cmdline_user_args():
		var m := preload("res://scripts/measure.gd").new()
		m.name = "Measure"
		add_child(m)
		m.start(self)

static func _keys() -> void:
	for action in KEYS:
		if InputMap.has_action(action):
			continue
		InputMap.add_action(action)
		for k in KEYS[action]:
			var ev := InputEventKey.new()
			ev.physical_keycode = k
			InputMap.action_add_event(action, ev)

func _ground(p: Vector3) -> Vector3:
	# The ground under p, under any roof (only the ground's collision layer).
	var space := get_world_3d().direct_space_state
	var hit := space.intersect_ray(PhysicsRayQueryParameters3D.create(p + Vector3.UP * 60.0, p + Vector3.DOWN * 60.0,
		Place.LAYER_GROUND))
	return hit["position"] + Vector3.UP * 0.05 if hit else p

func _morning(layout: Dictionary) -> void:
	# S1 Embun pagi: low warm sun through mist (environment.md, colour script). The sun and sky come from the place.
	var sky_c: Array = layout["sky"]["color"]
	var sky_color := Color(sky_c[0], sky_c[1], sky_c[2])
	var sky_mat := ProceduralSkyMaterial.new()
	sky_mat.sky_top_color = sky_color.darkened(0.25)
	sky_mat.sky_horizon_color = sky_color.lightened(0.35)
	sky_mat.ground_horizon_color = sky_mat.sky_horizon_color
	sky_mat.ground_bottom_color = Color(0.22, 0.24, 0.2)
	var sky := Sky.new()
	sky.sky_material = sky_mat
	var env := Environment.new()
	env.background_mode = Environment.BG_SKY
	env.sky = sky
	env.ambient_light_source = Environment.AMBIENT_SOURCE_SKY
	env.ambient_light_energy = 0.7
	env.reflected_light_source = Environment.REFLECTION_SOURCE_SKY
	env.tonemap_mode = Environment.TONE_MAPPER_AGX
	env.fog_enabled = true
	env.fog_light_color = Color(0.8, 0.82, 0.8)
	env.fog_density = 0.0035
	env.fog_sky_affect = 0.25
	env.fog_height = 1.0
	env.fog_height_density = 0.05
	env.glow_enabled = true
	env.glow_intensity = 0.4
	var we := WorldEnvironment.new()
	we.environment = env
	add_child(we)
	var s: Dictionary = layout["sun"]
	var sun := DirectionalLight3D.new()
	sun.name = "Sun"
	var to_sun := Vector3(s["to_sun"][0], s["to_sun"][1], s["to_sun"][2]).normalized()
	sun.basis = Basis.looking_at(-to_sun, Vector3.UP)
	sun.light_color = Color(s["color"][0], s["color"][1], s["color"][2])
	sun.light_energy = 1.25
	sun.shadow_enabled = true
	sun.directional_shadow_mode = DirectionalLight3D.SHADOW_PARALLEL_2_SPLITS
	sun.directional_shadow_max_distance = 60.0
	add_child(sun)

func _process(_delta: float) -> void:
	if _overlay == null:
		return
	if Input.is_action_just_pressed("debug_overlay"):
		_overlay.visible = not _overlay.visible
	if _overlay.visible:
		_overlay.text = "%d fps   %d draws   %dk triangles" % [Engine.get_frames_per_second(),
			Performance.get_monitor(Performance.RENDER_TOTAL_DRAW_CALLS_IN_FRAME),
			Performance.get_monitor(Performance.RENDER_TOTAL_PRIMITIVES_IN_FRAME) / 1000]
