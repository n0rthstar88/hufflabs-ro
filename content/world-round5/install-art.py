#!/usr/bin/env python3
"""Installs the small rathena.org art packs into a HuffLabs RO client folder.

usage: install-art.py <custom-downloads dir> <client dir> <grf file list>
  <grf file list>: `grftool list data.grf` output

Policy: FILL ONLY. A file is copied only if the client has no file by that name (official
data.grf or already loose), so these packs add missing art (sprites/icons for official items
that kRO never shipped art for) and can never overwrite official or HuffLabs files. The
exceptions are the REPLACE list below, chosen on purpose.
Skipped on purpose: 4th.grf (identical to our 2025 client), skating.grf (replaces an official
outfit), palette packs (content/world-round4 ships its own dyes), newcart.gpf (old GPF 0x102 format), sky motorcycle (needs a mount
system our client doesn't have), Domovoi/Wootan custom card art.
"""
import pathlib
import shutil
import subprocess
import sys
import tempfile
import zipfile

DL, CLIENT, GRFLIST = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2]), sys.argv[3]
UI = "유저인터페이스"

PACKS = ["battle_manuals [x5 new].zip", "blood_tears [new].zip", "bubble_gums [x5 new].zip", "bullets [x6 new].zip",
         "butterfly_wings [x5 new].zip", "card_sprites [x3 new].zip", "gemmed_sallet [new].zip", "sl_icons [x2 new].zip",
         "w0w_cardbmp [x6 new].zip", "w0w_items [x6 new].zip", "dullahan [white eyes].zip", "EREMES SCARF.zip",
         "Domovoi Card+Loot.zip", "Wootan Defender Card+Shield.zip",
         "fix wrong hair & gemmed sallet & thief rogue axe atking.rar"]
# (archive, file inside, client path) - deliberate replacements
REPLACE = [
    ("HD8BitDmgFont.zip", "숫자.spr", "sprite/이팩트/숫자.spr"),            # HD damage numbers
    ("HD8BitDmgFont.zip", "숫자.act", "sprite/이팩트/숫자.act"),
    ("Poring Cart.7z", "data/sprite/이팩트/손수레.spr", "sprite/이팩트/손수레.spr"),  # cart 1 -> Poring Cart
    ("Poring Cart.7z", "data/sprite/이팩트/손수레.act", "sprite/이팩트/손수레.act"),
    ("tank by dan.rar", "tank by dan/tank.spr", "sprite/이팩트/손수레4.spr"),      # cart 5 -> Mini Tank
    ("tank by dan.rar", "tank by dan/tank.act", "sprite/이팩트/손수레4.act"),
]


def disk(korean_path):
    """client folder name for a Korean path: cp949 bytes stored as Latin-1 characters"""
    return korean_path.encode("cp949").decode("latin-1")


def korean(name):
    """normalise an archive name (UTF-8 Korean or Latin-1 mojibake) to real Korean"""
    try:
        b = name.encode("latin-1")
        return b.decode("cp949") if any(c > 127 for c in b) else name
    except (UnicodeEncodeError, UnicodeDecodeError):
        return name


def entries(archive, tmp):
    """yields (inside-name in real Korean, local file path)"""
    if archive.suffix == ".zip":
        z = zipfile.ZipFile(archive)
        for i in z.infolist():
            if i.is_dir():
                continue
            name = i.filename
            if not i.flag_bits & 0x800:
                try:
                    name = name.encode("cp437").decode("cp949")
                except (UnicodeEncodeError, UnicodeDecodeError):
                    pass  # already readable
            name = korean(name)
            out = pathlib.Path(tmp) / str(abs(hash(name)))
            out.write_bytes(z.read(i))
            yield name, out
    else:
        tool = ["unrar", "x", "-o+", "-idq", str(archive), tmp + "/"] if archive.suffix == ".rar" else ["bsdtar", "-xf", str(archive), "-C", tmp]
        subprocess.run(tool, check=True)
        for p in pathlib.Path(tmp).rglob("*"):
            if p.is_file():
                yield korean(str(p.relative_to(tmp))), p


def target(name):
    """client path (under data/) for an archive entry, or None to skip"""
    parts = name.replace("\\", "/").split("/")
    low = [p.lower() for p in parts]
    for root in ("sprite", "texture"):
        if root in low:
            return "/".join(parts[low.index(root):])
    if parts[-1].lower().endswith((".txt", ".db")) or "card (custom)" in name.lower():
        return None
    if len(parts) >= 2 and low[-2] in ("collection", "item"):
        return f"texture/{UI}/{low[-2]}/{parts[-1]}"
    if parts[-1].lower().endswith((".spr", ".act")):
        folder = "몬스터" if low[-2] == "domovoi" else "아이템"
        return f"sprite/{folder}/{parts[-1]}"
    return None


official = set()
for line in open(GRFLIST, encoding="utf-8", errors="replace"):
    official.add(line.rstrip("\n").split("\t", 1)[-1].lower().replace("\\", "/"))

added = skipped = 0
for pack in PACKS:
    with tempfile.TemporaryDirectory() as tmp:
        n = 0
        for name, path in entries(DL / pack, tmp):
            t = target(name)
            if t is None:
                continue
            dst = CLIENT / "data" / disk(t)
            if f"data/{t}".lower() in official or dst.exists():
                skipped += 1
                continue
            dst.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(path, dst)
            n += 1
        added += n
        print(f"{pack}: {n} files added")
for archive, inside, t in REPLACE:
    with tempfile.TemporaryDirectory() as tmp:
        found = [p for name, p in entries(DL / archive, tmp) if name.replace("\\", "/") == inside]
        if len(found) != 1:
            raise SystemExit(f"{archive}: {inside} not found")
        dst = CLIENT / "data" / disk(t)
        dst.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(found[0], dst)
print(f"added {added}, skipped {skipped} the client already has, replaced {len(REPLACE)}")
