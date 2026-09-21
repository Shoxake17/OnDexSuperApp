"""GLB modellarini MOBIL uchun optimallashtirish (Blender skripti).

┌─ MUAMMO ──────────────────────────────────────────────────────────────┐
`book-cafe-maket` loyihasidagi modellar AI/fotogrammetriya orqali
yasalgan va SODDALASHTIRILMAGAN: bitta bistro stol 273 MB, butun
loyiha 2.87 GB.

Telefon bunday sahnani ocha olmaydi — xotira yetmaydi. Mobil me'yor:
butun sahna 30-80 MB, bitta buyum 0.5-3 MB.
└───────────────────────────────────────────────────────────────────────┘

Skript ikki rejimda ishlaydi:

    --mode analyze    hech narsani o'zgartirmaydi, faqat HISOBOT beradi
    --mode optimize   optimallashtirib yangi papkaga yozadi

ANALYZE birinchi yugurtiriladi: nima qancha og'irligini bilmasdan
maqsad qo'yish — ko'r-ko'rona ish.

Ishlatish (PowerShell):

    & "D:\\Blender\\blender.exe" --background --python `
        F:\\ChustApp\\scripts\\blender\\optimize_glb.py -- `
        --src "D:\\UnityHub\\book-cafe-maket" `
        --mode analyze

Optimizatsiya:

    ... --mode optimize --out "D:\\UnityHub\\book-cafe-optimized" `
        --max-tris 20000 --max-texture 1024

DIQQAT: asl fayllar HECH QACHON o'zgartirilmaydi — natija boshqa
papkaga yoziladi. Optimizatsiya yo'qotishli (qaytarib bo'lmaydi),
shuning uchun asl nusxa tegilmasligi shart.
"""

import argparse
import math
import os
import sys
import time

import bpy


# ── Yordamchilar ────────────────────────────────────────────────────


def parse_args() -> argparse.Namespace:
    """Blender argumentlari bizникidan `--` bilan ajratiladi."""
    argv = sys.argv
    argv = argv[argv.index("--") + 1:] if "--" in argv else []

    p = argparse.ArgumentParser(description="GLB optimizatsiyasi")
    p.add_argument("--src", required=True, help="GLB fayllar papkasi")
    p.add_argument("--out", help="natija papkasi (optimize rejimida)")
    p.add_argument("--mode", choices=["analyze", "optimize"], default="analyze")
    p.add_argument(
        "--max-tris",
        type=int,
        default=20000,
        help="bitta fayl uchun uchburchak chegarasi (standart 20000)",
    )
    p.add_argument(
        "--max-texture",
        type=int,
        default=1024,
        help="tekstura tomonining eng katta o'lchami (standart 1024)",
    )
    p.add_argument(
        "--planar-angle",
        type=float,
        default=1.0,
        help=(
            "tekis yuzalarni birlashtirish burchagi, daraja (standart 1). "
            "AI/fotogrammetriya modellarida 'tekis' yuza aslida notekis "
            "bo'ladi va kichik burchak hech narsa topmaydi - bunday "
            "hollarda 5-15 sinab ko'riladi"
        ),
    )
    return p.parse_args(argv)


def clear_scene() -> None:
    """Sahnani TO'LIQ tozalaydi.

    Faqat obyektlarni o'chirish YETARLI EMAS: mesh va tekstura
    ma'lumotlari xotirada qolib, keyingi fayllarga qo'shilib ketadi va
    hisobot buziladi (hamda xotira asta-sekin to'ladi).
    """
    bpy.ops.wm.read_factory_settings(use_empty=True)
    for block in (
        bpy.data.meshes,
        bpy.data.materials,
        bpy.data.images,
        bpy.data.textures,
    ):
        for item in list(block):
            block.remove(item, do_unlink=True)


def scene_triangles() -> int:
    total = 0
    for obj in bpy.data.objects:
        if obj.type != "MESH":
            continue
        mesh = obj.data
        mesh.calc_loop_triangles()
        total += len(mesh.loop_triangles)
    return total


def texture_report() -> list[tuple[str, int, int]]:
    out = []
    for img in bpy.data.images:
        if img.size[0] and img.size[1]:
            out.append((img.name, img.size[0], img.size[1]))
    return out


def mb(path: str) -> float:
    return os.path.getsize(path) / (1024 * 1024)


def glb_files(src: str) -> list[str]:
    """Papkadagi barcha GLB — ichki papkalar bilan.

    Bo'sh (0 baytli) fayllar TASHLAB KETILADI: loyihada bir nechta
    shunday fayl bor (`tv.glb`, `white_shelf.glb`) va ular import
    qilinganda Blender xato beradi.
    """
    found = []
    for root, _dirs, files in os.walk(src):
        # Godot keshi — bu yerda asl model yo'q, faqat import natijasi.
        if os.sep + ".godot" in root:
            continue
        for f in files:
            if f.lower().endswith((".glb", ".gltf")):
                full = os.path.join(root, f)
                if os.path.getsize(full) > 1024:
                    found.append(full)
    return sorted(found)


# ── Optimizatsiya qadamlari ─────────────────────────────────────────


def decimate_to(max_tris: int, planar_angle: float = 1.0) -> tuple[int, int]:
    """Uchburchaklarni chegaraga tushiradi.

    Nisbat HAR OBYEKT uchun alohida hisoblanadi: sahnada bitta og'ir
    va bir nechta yengil obyekt bo'lsa, umumiy nisbat yengillarini
    keraksiz buzardi.

    `COLLAPSE` — chekka qisqartirish: shaklni eng yaxshi saqlaydigan
    usul. `UNSUBDIV` tezroq, lekin natija ko'pincha buzuq chiqadi.
    """
    before = scene_triangles()
    meshes = [o for o in bpy.data.objects if o.type == "MESH"]
    if not meshes:
        return 0, 0

    # Chegara obyektlar orasida ULARNING OG'IRLIGIGA QARAB bo'linadi —
    # teng bo'linsa, mayda detal (masalan piyola) ham katta devor bilan
    # bir xil "byudjet" olardi.
    per_obj = {}
    for obj in meshes:
        obj.data.calc_loop_triangles()
        per_obj[obj.name] = len(obj.data.loop_triangles)
    total = sum(per_obj.values()) or 1

    # ── Bir xil mesh'ni ULASHGAN obyektlar ───────────────────────────
    # Blender ko'p egali (multi-user) ma'lumotga modifikator
    # QO'LLAMAYDI. Modeldan bir nechta nusxa qo'yilgan bo'lsa — kafedagi
    # bir xil kreslolar aynan shunday — `modifier_apply` xato beradi.
    #
    # Avval bu xato jimgina yutilardi va model ASL og'irligida
    # qolaverardi: Hi3D modeli 20 000 byudjetga qaramay 55 379
    # uchburchak bo'lib qoldi. Buni faqat sahnani qayta sanaganda
    # payqash mumkin edi — skript esa "bajarildi" deb hisobot berardi.
    #
    # Yechim: har bir NOYOB mesh bir marta soddalashtiriladi, so'ng
    # natija o'sha mesh'ni ulashgan hamma obyektga beriladi. Nusxalash
    # saqlanadi — xotirada bitta mesh yotadi.
    by_data: dict = {}
    for obj in meshes:
        by_data.setdefault(obj.data.name, []).append(obj)

    for objs in by_data.values():
        first = objs[0]
        tris = per_obj[first.name]
        if tris == 0:
            continue
        budget = max(200, int(max_tris * tris / total))
        if tris <= budget:
            continue

        # Modifikator qo'llanishi uchun ma'lumot yagona egali bo'lishi
        # kerak. Nusxa faqat shu bosqichda olinadi.
        if first.data.users > 1:
            first.data = first.data.copy()

        bpy.ops.object.select_all(action="DESELECT")
        first.select_set(True)
        bpy.context.view_layer.objects.active = first

        # ┌─ CHO'QQILARNI PAYVANDLASH SINALDI VA RAD ETILDI ───────────┐
        # `remove_doubles` glTF takrorlagan cho'qqilarni birlashtiradi
        # va nazariy jihatdan soddalashtirishni yaxshilashi kerak edi.
        # Amalda kitob shkafi BUTUNLAY buzildi: yupqa devorlarning ichki
        # va tashqi yuzalari bir-biriga yopishib, tokchalar yo'qoldi va
        # shkaf yaxlit plitaga aylandi.
        #
        # Shuning uchun bu qadam qo'shilmaydi. Yupqa devorli mebelda
        # payvandlash xavfli.
        # └────────────────────────────────────────────────────────────┘

        # ── 1-BOSQICH: TEKIS YUZALARNI TOZALASH (PLANAR) ─────────────
        # `COLLAPSE` tekis panelni buzadi: u chekkalarni qisqartirganda
        # katta tekis yuzada uzun, qiyshiq uchburchaklar hosil qiladi va
        # panel "erib ketgan"dek ko'rinadi. Kitob shkafi aynan shundan
        # zarar ko'rgan edi — 1 988 760 dan 13 999 ga tushirilganda
        # tekis tokchalar burma-burma bo'lib qolgan.
        #
        # `DISSOLVE` (planar) esa BIR TEKISLIKDAGI qo'shni uchburchaklarni
        # birlashtiradi. Shakl umuman o'zgarmaydi - faqat ortiqcha
        # bo'linishlar yo'qoladi. Bunday modellarda (shkaf, stol, devor)
        # bu bosqichning o'zi hajmni o'nlab barobar tushiradi.
        #
        # 1 daraja: deyarli qat'iy tekislik talab qilinadi, ya'ni
        # yumaloq qismlar (o'simlik barglari) tegilmaydi.
        planar = first.modifiers.new(name="OnDexPlanar", type="DECIMATE")
        planar.decimate_type = "DISSOLVE"
        planar.angle_limit = math.radians(planar_angle)
        planar.use_dissolve_boundaries = False
        try:
            bpy.ops.object.modifier_apply(modifier=planar.name)
        except RuntimeError as exc:
            print(f"    OGOHLANTIRISH: {first.name} planar bosqichi o'tmadi ({exc})")
            first.modifiers.remove(planar)

        # ── 2-BOSQICH: QOLGANINI QISQARTIRISH (COLLAPSE) ─────────────
        # Planar bosqichdan keyin qancha qolganini QAYTA sanaymiz:
        # byudjetga allaqachon sig'gan bo'lsa, shaklni yana buzishning
        # hojati yo'q.
        first.data.calc_loop_triangles()
        tris_now = len(first.data.loop_triangles)

        if tris_now > budget:
            mod = first.modifiers.new(name="OnDexDecimate", type="DECIMATE")
            mod.decimate_type = "COLLAPSE"
            mod.ratio = budget / tris_now
            bpy.context.view_layer.objects.active = first
            try:
                bpy.ops.object.modifier_apply(modifier=mod.name)
            except RuntimeError as exc:
                # Xato endi KO'RINADI: jim qolish natijani soxtalashtiradi.
                print(f"    OGOHLANTIRISH: {first.name} soddalashmadi ({exc})")
                first.modifiers.remove(mod)
                continue

        # Qolgan nusxalarga yengillashgan mesh beriladi.
        for other in objs[1:]:
            other.data = first.data

    return before, scene_triangles()


def shrink_textures(max_size: int) -> int:
    """Katta teksturalarni kichraytiradi. Nechta o'zgargani qaytadi."""
    changed = 0
    for img in bpy.data.images:
        w, h = img.size
        if w <= max_size and h <= max_size:
            continue
        scale = max_size / max(w, h)
        img.scale(max(1, int(w * scale)), max(1, int(h * scale)))
        changed += 1
    return changed


# ── Asosiy oqim ─────────────────────────────────────────────────────


def analyze(files: list[str]) -> None:
    print("\n" + "=" * 78)
    print("O'LCHOV HISOBOTI (hech narsa o'zgartirilmadi)")
    print("=" * 78)
    print(f"{'Fayl':<48}{'MB':>9}{'Uchburchak':>14}")
    print("-" * 78)

    total_mb = 0.0
    total_tris = 0
    big_textures = 0

    for path in files:
        clear_scene()
        try:
            bpy.ops.import_scene.gltf(filepath=path)
        except Exception as exc:  # noqa: BLE001 — hisobot to'xtamasin
            print(f"{os.path.basename(path):<48}{'XATO':>9}  {exc}")
            continue
        size = mb(path)
        tris = scene_triangles()
        total_mb += size
        total_tris += tris
        for _name, w, h in texture_report():
            if max(w, h) > 1024:
                big_textures += 1
        print(f"{os.path.basename(path)[:47]:<48}{size:>9.1f}{tris:>14,}")

    print("-" * 78)
    print(f"{'JAMI':<48}{total_mb:>9.1f}{total_tris:>14,}")
    print(f"\n1024 dan katta tekstura: {big_textures} ta")
    print(
        "\nMobil me'yor: butun sahna 30-80 MB, 300-800 ming uchburchak.\n"
        "Yuqoridagi raqamlar shundan qancha oshganini ko'rsatadi."
    )


def optimize(files: list[str], out: str, max_tris: int, max_texture: int, planar_angle: float = 1.0) -> None:
    os.makedirs(out, exist_ok=True)
    print("\n" + "=" * 78)
    print("OPTIMIZATSIYA")
    print(f"chegara: {max_tris:,} uchburchak / fayl, tekstura {max_texture}px")
    print("=" * 78)
    print(f"{'Fayl':<40}{'MB oldin':>10}{'MB keyin':>10}{'Uchburchak':>22}")
    print("-" * 78)

    before_total = 0.0
    after_total = 0.0

    for path in files:
        clear_scene()
        try:
            bpy.ops.import_scene.gltf(filepath=path)
        except Exception as exc:  # noqa: BLE001
            print(f"{os.path.basename(path)[:39]:<40}  XATO: {exc}")
            continue

        tris_before, tris_after = decimate_to(max_tris, planar_angle)
        shrink_textures(max_texture)

        # Nomlar to'qnashmasligi uchun ichki papka tuzilmasi saqlanadi.
        #
        # Taglik sifatida fayllarning emas, ularning PAPKALARINING
        # umumiy qismi olinadi. Avval `commonpath(files)` ishlatilgan
        # edi va bitta fayl berilganda u faylning O'ZINI qaytarardi:
        # `relpath(path, path)` = "." va nom `..glb` bo'lib chiqardi.
        base = os.path.commonpath([os.path.dirname(f) for f in files])
        rel = os.path.relpath(path, base)
        dst = os.path.join(out, os.path.splitext(rel)[0] + ".glb")
        os.makedirs(os.path.dirname(dst), exist_ok=True)

        # Draco siqish ATAYLAB YOQILMAGAN.
        #
        # U faylni yana ~2 barobar kichraytirardi, lekin Godot'ning
        # glTF importi Draco'ni ishonchli ochmaydi — natijada model
        # umuman ko'rinmay qolishi mumkin. Hajmni biz decimate va
        # tekstura orqali tushiramiz, ular har qanday dvigatelda
        # ishlaydi.
        bpy.ops.export_scene.gltf(
            filepath=dst,
            export_format="GLB",
            export_draco_mesh_compression_enable=False,
            export_apply=True,
        )

        size_before = mb(path)
        size_after = mb(dst)
        before_total += size_before
        after_total += size_after
        print(
            f"{os.path.basename(path)[:39]:<40}"
            f"{size_before:>10.1f}{size_after:>10.1f}"
            f"{tris_before:>10,} → {tris_after:<9,}"
        )

    print("-" * 78)
    saved = before_total - after_total
    ratio = (before_total / after_total) if after_total else 0
    print(f"{'JAMI':<40}{before_total:>10.1f}{after_total:>10.1f}")
    print(f"\nTejaldi: {saved:,.1f} MB  ({ratio:.1f}x kichrayди)")
    print(f"Natija papkasi: {out}")


def main() -> None:
    args = parse_args()
    files = glb_files(args.src)
    if not files:
        print(f"XATO: {args.src} da GLB fayl topilmadi")
        return

    print(f"\n{len(files)} ta GLB topildi: {args.src}")
    start = time.time()

    if args.mode == "analyze":
        analyze(files)
    else:
        if not args.out:
            print("XATO: --out ko'rsatilmagan")
            return
        optimize(files, args.out, args.max_tris, args.max_texture, args.planar_angle)

    print(f"\nVaqt: {time.time() - start:.1f} s")


if __name__ == "__main__":
    main()
