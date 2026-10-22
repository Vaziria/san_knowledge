class_name Place
extends Node3D

# The game's copy of a Blender place, exported by project/bargawa/scripts/game/<name>.py (run.ps1 game <name>):
#   <name>.glb   one node per distinct mesh (K0001 ...) at the origin, with plain glTF materials
#   <name>.json  where each mesh stands (Godot transforms), its kind, collider and draw range; the sun, the sky, the
#                walkable area and where the player starts
# A mesh placed more than once is drawn as MultiMeshes, one per CHUNK-metre square, so a phone isn't asked for
# thousands of draw calls; a mesh placed once stays a MeshInstance3D and keeps Godot's automatic LODs.
# Blender (x, y, z) is Godot (x, z, -y): north is -Z, south (the lake) is +Z.

const CHUNK := 16.0
const NO_SHADOW := ["grass", "ground", "water", "far"]
# Collision layers: the ground on its own, so a ray can find it under a roof (spawning, the camera's floor).
const LAYER_SOLID := 1
const LAYER_GROUND := 2

var layout: Dictionary
var _walk := PackedByteArray()
var _walk_x0 := 0.0
var _walk_y0 := 0.0
var _walk_cell := 1.0
var _walk_cols := 0
var _walk_rows := 0

func load_place(json_path: String) -> void:
	layout = (load(json_path) as JSON).data
	var lib: Node = (load(json_path.get_basename() + ".glb") as PackedScene).instantiate()
	var meshes := {}
	for mi in lib.find_children("K*", "MeshInstance3D", true, false):
		meshes[String(mi.name)] = (mi as MeshInstance3D).mesh
	lib.free()
	for id in layout["meshes"]:
		var e: Dictionary = layout["meshes"][id]
		var mesh: Mesh = meshes.get(id)
		if mesh == null:
			push_warning("place: no mesh %s (%s) in the GLB" % [id, e["name"]])
			continue
		var xforms: Array[Transform3D] = []
		for a in e["xforms"]:
			xforms.append(_xform(a))
		_draw(mesh, e, xforms)
		_collide(mesh, e, xforms)
	_load_walk(layout["walk"])

static func _xform(a: Array) -> Transform3D:
	return Transform3D(Basis(Vector3(a[0], a[1], a[2]), Vector3(a[3], a[4], a[5]), Vector3(a[6], a[7], a[8])),
		Vector3(a[9], a[10], a[11]))

func _draw(mesh: Mesh, e: Dictionary, xforms: Array[Transform3D]) -> void:
	var kind: String = e["kind"]
	var shadow := GeometryInstance3D.SHADOW_CASTING_SETTING_OFF if kind in NO_SHADOW else GeometryInstance3D.SHADOW_CASTING_SETTING_ON
	var range_end: float = e.get("range", 0.0)
	if kind == "water":
		mesh = mesh.duplicate() as Mesh
		mesh.surface_set_material(0, _water_material())
	if xforms.size() == 1:
		var mi := MeshInstance3D.new()
		mi.name = String(e["name"]).validate_node_name()
		mi.mesh = mesh
		mi.transform = xforms[0]
		mi.cast_shadow = shadow
		_set_range(mi, range_end)
		add_child(mi)
		return
	var chunks := {}
	for t in xforms:
		var k := Vector2i(floori(t.origin.x / CHUNK), floori(t.origin.z / CHUNK))
		if not chunks.has(k):
			chunks[k] = []
		chunks[k].append(t)
	for k in chunks:
		var list: Array = chunks[k]
		var mm := MultiMesh.new()
		mm.transform_format = MultiMesh.TRANSFORM_3D
		mm.mesh = mesh
		mm.instance_count = list.size()
		for i in list.size():
			mm.set_instance_transform(i, list[i])
		var mmi := MultiMeshInstance3D.new()
		mmi.name = "%s_%d_%d" % [String(e["name"]).validate_node_name(), k.x, k.y]
		mmi.multimesh = mm
		mmi.cast_shadow = shadow
		_set_range(mmi, range_end)
		add_child(mmi)

func _set_range(g: GeometryInstance3D, range_end: float) -> void:
	if range_end <= 0.0:
		return
	g.visibility_range_end = range_end
	g.visibility_range_end_margin = range_end * 0.1

func _water_material() -> StandardMaterial3D:
	# A stand-in for the lake's water shader (milestone 4): dark, smooth, reflecting the sky.
	var m := StandardMaterial3D.new()
	m.albedo_color = Color(0.06, 0.1, 0.11)
	m.roughness = 0.04
	m.metallic_specular = 0.7
	return m

func _collide(mesh: Mesh, e: Dictionary, xforms: Array[Transform3D]) -> void:
	if not e.has("collide"):
		return
	var body := StaticBody3D.new()
	body.name = "Collide_" + String(e["name"]).validate_node_name()
	body.collision_layer = LAYER_GROUND if e["kind"] == "ground" else LAYER_SOLID
	body.collision_mask = 0
	add_child(body)
	var c = e["collide"]
	if c is String:
		var shape := mesh.create_trimesh_shape()
		for t in xforms:
			var cs := CollisionShape3D.new()
			cs.shape = shape
			cs.transform = t
			body.add_child(cs)
	else:
		var cyl := CylinderShape3D.new()
		cyl.radius = c["r"]
		cyl.height = c["h"]
		for t in xforms:
			var cs := CollisionShape3D.new()
			cs.shape = cyl
			cs.position = t.origin + Vector3(0.0, cyl.height * 0.5, 0.0)
			body.add_child(cs)

func _load_walk(w: Dictionary) -> void:
	_walk_x0 = w["x0"]
	_walk_y0 = w["y0"]
	_walk_cell = w["cell"]
	_walk_cols = w["cols"]
	_walk_rows = w["rows"]
	_walk.resize(_walk_cols * _walk_rows)
	var r := 0
	for row in w["data"]:
		var s: String = row
		for c in _walk_cols:
			_walk[r * _walk_cols + c] = 1 if s[c] == "1" else 0
		r += 1

func is_walkable(p: Vector3) -> bool:
	# The walkable area from the export (the clearing and the slope to the lake, not the water).
	var c := floori((p.x - _walk_x0) / _walk_cell)
	var r := floori((-p.z - _walk_y0) / _walk_cell)
	if c < 0 or r < 0 or c >= _walk_cols or r >= _walk_rows:
		return false
	return _walk[r * _walk_cols + c] == 1

func spawn() -> Dictionary:
	var s: Dictionary = layout["spawn"]
	return {"position": Vector3(s["x"], 0.0, -s["y"]), "face": s["face"]}
