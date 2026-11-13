#!/usr/bin/env python3
"""Adds the round-4 client files to a HuffLabs RO client folder. Run content/world-round1's
build-client.py first (it writes the plain-Lua accessory tables this one appends to), and
content/coin-shop/build-client.py for the item names (it reads headgears.tsv).

usage: build-client.py <custom-downloads dir> <client dir>
- extra dyes: 553-ClothesDyes-251HairDyes.grf -> <client>/hlro_dyes.grf, listed in DATA.INI after
  data.grf (official palettes win; the pack only adds colour numbers)
- 316 costume headgears from Mihael's pack (headgears.tsv): sprites as _hlro_<name>, icons
  hlro_<name>.bmp, look ids in accessoryid/accname.lub
"""
import pathlib
import shutil
import sys

DL, CLIENT = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2])
HERE = pathlib.Path(__file__).resolve().parent
UI = "À¯ÀúÀÎÅÍÆäÀÌ½º"
ACC, M, W = "¾Ç¼¼»ç¸®", "³²", "¿©"
DATAINFO = CLIENT / "data" / "luafiles514" / "lua files" / "datainfo"

# 1. dyes
src = DL / "553-ClothesDyes-251HairDyes.grf"
dst = CLIENT / "hlro_dyes.grf"
if not dst.exists() or dst.stat().st_size != src.stat().st_size:
    shutil.copyfile(src, dst)
ini = (CLIENT / "DATA.INI").read_text(encoding="latin-1")
if "hlro_dyes.grf" not in ini:
    lines = ini.replace("\r\n", "\n").rstrip("\n").split("\n")
    nums = [int(l.split("=")[0]) for l in lines if "=" in l and l.split("=")[0].isdigit()]
    lines.append(f"{max(nums) + 1}=hlro_dyes.grf")
    (CLIENT / "DATA.INI").write_text("\r\n".join(lines) + "\r\n", encoding="latin-1")
print("dyes: hlro_dyes.grf + DATA.INI")

# 2. headgears (Mihael's pack + the Valentine Ring and Flames recolours)
SRC = {"mihael": (DL / "mihael_x" / "Pack Custom Itens Mihael", "item", "item"),
       "Valentine_Ring": (DL / "extra_x" / "Valentine_Ring", f"texture/{UI}/item", f"texture/{UI}/collection"),
       "flame_custom": (DL / "extra_x" / "flame_custom", "item", "collection")}
rows = [l.split("\t") for l in (HERE / "headgears.tsv").read_text().splitlines() if l and not l.startswith("#")]
listing = {}
def find(folder, fname):
    if folder not in listing:
        listing[folder] = {p.name.lower(): p for p in folder.iterdir()}
    return listing[folder][fname.lower()]
for _, _, name, _, _, src in rows:
    root, item_dir, coll_dir = SRC[src]
    for sex in (M, W):
        d = CLIENT / "data" / "sprite" / ACC / sex
        d.mkdir(parents=True, exist_ok=True)
        for ext in ("spr", "act"):
            folder = root / "sprite" / ACC / sex
            try:
                spr = find(folder, f"{sex}_{name}.{ext}")
            except KeyError:  # a few pack files carry the other sex's prefix inside the right folder
                spr = find(folder, f"{(W if sex == M else M)}_{name}.{ext}")
            shutil.copyfile(spr, d / f"{sex}_hlro_{name}.{ext}")
    for kind, sub in (("item", item_dir), ("collection", coll_dir)):
        d = CLIENT / "data" / "texture" / UI / kind
        d.mkdir(parents=True, exist_ok=True)
        try:
            icon = find(root / sub, f"{name}.bmp")
        except KeyError:
            icon = find(root / item_dir, f"{name}.bmp")
        shutil.copyfile(icon, d / f"hlro_{name}.bmp")

ids = (DATAINFO / "accessoryid.lub").read_text(encoding="latin-1")
accn = (DATAINFO / "accname.lub").read_text(encoding="latin-1")
if "ACCESSORY_HLRO_SAIYAN_AURA" not in ids:
    raise SystemExit("run content/world-round1/build-client.py first")
add_i, add_n = [], []
for _, view, name, _, _, _ in rows:
    key = f"ACCESSORY_HLRO_{name.upper().replace('-', '_')}"
    if f"\t{key} = " in ids:
        continue
    if f"= {view},\n" in ids:
        raise SystemExit(f"view id {view} already used")
    add_i.append(f"\t{key} = {view},\n")
    add_n.append(f'\t[ACCESSORY_IDs.{key}] = "_hlro_{name}",\n')
(DATAINFO / "accessoryid.lub").write_text(ids[:-2] + "".join(add_i) + "}\n", encoding="latin-1")
(DATAINFO / "accname.lub").write_text(accn[:-2] + "".join(add_n) + "}\n", encoding="latin-1")
print(f"headgears: {len(rows)} (sprites, icons, {len(add_i)} look ids added)")
