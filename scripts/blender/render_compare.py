"""GLB modelni rasmga chizadi — optimizatsiya natijasini KO'Z bilan
tekshirish uchun.

Raqamlar (hajm, uchburchak) shakl buzilganini ko'rsatmaydi: 122 barobar
qisqargan stol chiroyli qolishi ham, tanib bo'lmas holga kelishi ham
mumkin. Buni faqat rasmga qarab bilish mumkin.

Ishlatish:

    blender --background --python render_compare.py -- \\
        --model "D:/.../panjara.glb" --out "D:/.../panjara.png"
"""

import argparse
import math
import os
import sys

import bpy
from mathutils import Vector


def parse_args() -> argparse.Namespace:
    argv = sys.argv
    argv = argv[argv.index("--") + 1:] if "--" in argv else []
    p = argparse.ArgumentParser()
    p.add_argument("--model", required=True)
    p.add_argument("--out", required=True)
    p.add_argument("--size", type=int, default=700)
    return p.parse_args(argv)


def clear() -> None:
    bpy.ops.wm.read_factory_settings(use_empty=True)


def scene_bounds() -> tuple[Vector, float]:
    """Barcha mesh'lar markazi va radiusi.

    Kamerani MODELGA MOSLAB qo'yish uchun: qat'iy masofa qo'yilsa,
    kichik model nuqtaday, katta model esa kadrga sig'may qolardi.
    """
    pts: list[Vector] = []
    for obj in bpy.data.objects:
        if obj.type != "MESH":
            continue
        for corner in obj.bound_box:
            pts.append(obj.matrix_world @ Vector(corner))
    if not pts:
        return Vector((0, 0, 0)), 1.0

    lo = Vector((min(p.x for p in pts), min(p.y for p in pts), min(p.z for p in pts)))
    hi = Vector((max(p.x for p in pts), max(p.y for p in pts), max(p.z for p in pts)))
    center = (lo + hi) / 2
    radius = max((hi - lo).length / 2, 0.001)
    return center, radius


def setup_camera(center: Vector, radius: float) -> None:
    cam_data = bpy.data.cameras.new("Cam")
    cam = bpy.data.objects.new("Cam", cam_data)
    bpy.context.scene.collection.objects.link(cam)
    bpy.context.scene.camera = cam

    # Old tomondan, biroz yuqoridan — buyum shakli eng yaxshi
    # ko'rinadigan burchak.
    dist = radius * 3.0
    cam.location = center + Vector(
        (dist * 0.7, -dist * 0.8, dist * 0.5)
    )
    direction = center - cam.location
    cam.rotation_euler = direction.to_track_quat("-Z", "Y").to_euler()


def setup_light(center: Vector, radius: float) -> None:
    # Ikki yorug'lik: asosiy + to'ldiruvchi. Bitta bo'lsa modelning
    # yarmi qop-qora chiqadi va buzilgan joyni ko'rib bo'lmaydi.
    for i, (offset, energy) in enumerate(
        [((1, -1, 1.5), 3.0), ((-1.2, 0.8, 0.6), 1.2)]
    ):
        data = bpy.data.lights.new(f"L{i}", type="SUN")
        data.energy = energy
        light = bpy.data.objects.new(f"L{i}", data)
        bpy.context.scene.collection.objects.link(light)
        light.location = center + Vector(offset) * radius * 4
        d = center - light.location
        light.rotation_euler = d.to_track_quat("-Z", "Y").to_euler()


def render(out_path: str, size: int) -> None:
    scene = bpy.context.scene
    scene.render.engine = "BLENDER_WORKBENCH"  # tez, materialsiz ham ishlaydi
    scene.render.resolution_x = size
    scene.render.resolution_y = size
    scene.render.film_transparent = False
    scene.render.filepath = out_path
    scene.render.image_settings.file_format = "PNG"

    shading = scene.display.shading
    shading.light = "STUDIO"
    shading.color_type = "TEXTURE"
    shading.show_shadows = True
    shading.show_cavity = True  # qirralarni ajratib ko'rsatadi

    bpy.ops.render.render(write_still=True)


def main() -> None:
    args = parse_args()
    clear()
    bpy.ops.import_scene.gltf(filepath=args.model)

    tris = 0
    for obj in bpy.data.objects:
        if obj.type == "MESH":
            obj.data.calc_loop_triangles()
            tris += len(obj.data.loop_triangles)

    center, radius = scene_bounds()
    setup_camera(center, radius)
    setup_light(center, radius)

    os.makedirs(os.path.dirname(args.out), exist_ok=True)
    render(args.out, args.size)

    print(
        f"CHIZILDI | {os.path.basename(args.model)} | "
        f"{tris:,} uchburchak | radius {radius:.2f} | {args.out}"
    )


if __name__ == "__main__":
    main()
