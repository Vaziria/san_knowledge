class_name Player
extends CharacterBody3D

# Ramabargawa as the player (docs/summaries/models/ramabargawa.md). Until his model is built (milestone 2) he is a
# stand-in: a figure his height (1.80 m, nearer Javanese stature: blender/places/jatisrana.md). Third person: the camera
# follows behind on a spring arm.
# Keyboard and mouse for the Windows preview; touch controls come with milestone 3.
# He walks, never runs (a grief story); he climbs single steps (the stone stairs) and keeps to the place's walkable area.

const WALK := 1.6            # m/s
const TURN := 8.0            # how fast the body turns to where he walks
const STEP := 0.32           # highest step he climbs
const GRAVITY := 9.8
const HEIGHT := 1.8
const MOUSE := 0.0025

var place: Place
var auto_dir := Vector3.ZERO     # set by the measuring autopilot: a world direction to walk in
var cam_yaw := 0.0
var cam_pitch := -0.22
var _body: Node3D
var _pivot: Node3D
var camera: Camera3D

func _ready() -> void:
	floor_snap_length = 0.45
	floor_max_angle = deg_to_rad(50.0)
	collision_mask = Place.LAYER_SOLID | Place.LAYER_GROUND
	var shape := CollisionShape3D.new()
	var capsule := CapsuleShape3D.new()
	capsule.radius = 0.28
	capsule.height = HEIGHT
	shape.shape = capsule
	shape.position.y = HEIGHT * 0.5
	add_child(shape)
	_body = Node3D.new()
	add_child(_body)
	_body.add_child(_stand_in())
	_pivot = Node3D.new()
	_pivot.position.y = 1.55
	add_child(_pivot)
	var arm := SpringArm3D.new()
	arm.spring_length = 3.8
	arm.margin = 0.2
	arm.collision_mask = Place.LAYER_SOLID | Place.LAYER_GROUND
	arm.add_excluded_object(get_rid())
	_pivot.add_child(arm)
	camera = Camera3D.new()
	camera.fov = 62.0
	camera.near = 0.1
	camera.far = 6000.0
	arm.add_child(camera)

func _stand_in() -> Node3D:
	# A dark figure with a lighter band for the kain, so the size and the walk read against the place.
	var root := Node3D.new()
	var skin := StandardMaterial3D.new()
	skin.albedo_color = Color(0.36, 0.25, 0.18)
	var kain := StandardMaterial3D.new()
	kain.albedo_color = Color(0.16, 0.13, 0.11)
	var body := MeshInstance3D.new()
	var cap := CapsuleMesh.new()
	cap.radius = 0.24
	cap.height = HEIGHT - 0.25
	body.mesh = cap
	body.position.y = (HEIGHT - 0.25) * 0.5
	body.material_override = skin
	root.add_child(body)
	var head := MeshInstance3D.new()
	var sphere := SphereMesh.new()
	sphere.radius = 0.12
	sphere.height = 0.24
	head.mesh = sphere
	head.position.y = HEIGHT - 0.12
	head.material_override = skin
	root.add_child(head)
	var band := MeshInstance3D.new()
	var cyl := CylinderMesh.new()
	cyl.top_radius = 0.27
	cyl.bottom_radius = 0.3
	cyl.height = 0.55
	band.mesh = cyl
	band.position.y = 0.75
	band.material_override = kain
	root.add_child(band)
	var nose := MeshInstance3D.new()       # which way he faces
	var box := BoxMesh.new()
	box.size = Vector3(0.06, 0.06, 0.1)
	nose.mesh = box
	nose.position = Vector3(0.0, HEIGHT - 0.14, 0.13)
	nose.material_override = skin
	root.add_child(nose)
	return root

func face(direction: Vector3) -> void:
	# Turn him and the camera to look along a world direction.
	var y := atan2(direction.x, direction.z)
	_body.rotation.y = y
	cam_yaw = y + PI

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventMouseButton and event.pressed and Input.mouse_mode != Input.MOUSE_MODE_CAPTURED:
		Input.mouse_mode = Input.MOUSE_MODE_CAPTURED
	elif event.is_action_pressed("ui_cancel"):
		Input.mouse_mode = Input.MOUSE_MODE_VISIBLE
	elif event is InputEventMouseMotion and Input.mouse_mode == Input.MOUSE_MODE_CAPTURED:
		cam_yaw -= event.relative.x * MOUSE
		cam_pitch = clampf(cam_pitch - event.relative.y * MOUSE, -1.1, 0.35)

func _physics_process(delta: float) -> void:
	var dir := auto_dir
	if dir == Vector3.ZERO:
		var input := Input.get_vector("move_left", "move_right", "move_forward", "move_back")
		dir = Basis(Vector3.UP, cam_yaw) * Vector3(input.x, 0.0, input.y)
	var motion := _keep_walkable(dir.limit_length(1.0) * WALK, delta)
	velocity.x = motion.x
	velocity.z = motion.z
	velocity.y = 0.0 if is_on_floor() else velocity.y - GRAVITY * delta
	_climb(Vector3(motion.x, 0.0, motion.z) * delta)
	move_and_slide()
	if motion.length() > 0.05:
		var target := atan2(motion.x, motion.z)
		_body.rotation.y = lerp_angle(_body.rotation.y, target, clampf(TURN * delta, 0.0, 1.0))
	_pivot.rotation = Vector3(cam_pitch, cam_yaw, 0.0)

func _keep_walkable(v: Vector3, delta: float) -> Vector3:
	# Stop at the edge of the walkable area; slide along it when only one direction is blocked.
	if place == null or v == Vector3.ZERO:
		return v
	var ahead := 0.35 / maxf(v.length(), 0.01)
	if place.is_walkable(global_position + v * ahead):
		return v
	var vx := Vector3(v.x, 0.0, 0.0)
	if vx.length() > 0.01 and place.is_walkable(global_position + vx * ahead):
		return vx
	var vz := Vector3(0.0, 0.0, v.z)
	if vz.length() > 0.01 and place.is_walkable(global_position + vz * ahead):
		return vz
	return Vector3.ZERO

func _climb(motion: Vector3) -> void:
	# Up a single step: blocked ahead, but free one step higher.
	if motion.length() < 0.001 or not is_on_floor():
		return
	var t := global_transform
	var ahead := motion.normalized() * maxf(motion.length(), 0.08)
	if not test_move(t, ahead):
		return
	var up := Vector3(0.0, STEP, 0.0)
	if test_move(t, up) or test_move(t.translated(up), ahead):
		return
	global_position += up
