#!/usr/bin/env python3
"""Adds the round-3 client files to a HuffLabs RO client folder. Run content/world-round1's
build-client.py first: this one appends to the plain-Lua npcidentity/jobname it writes.

usage: build-client.py <picks dir> <client dir> <grf file list> <season>
  <grf file list>: `grftool list data.grf` output (for Prontera Seasons' Korean file names)
  <season>: spring | summer | fall | winter

- Prontera Seasons (Olrox): the season's prontera map + its models/textures
- Poring Family: 25 monster sprites as data/sprite/<monster>/hlro_<name>.spr, ids 31000-31024
- 100 NPCs pack: the 94 kept looks as data/sprite/npc/hlro_<name>.spr, ids 19200+ (npc_looks.tsv)
Art is copied from the quarantine folder and never committed.
"""
import pathlib
import shutil
import subprocess
import sys
import tempfile

PICKS, CLIENT = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2])
GRFLIST, SEASON = sys.argv[3], sys.argv[4]
HERE = pathlib.Path(__file__).resolve().parent
MONSTER = "¸ó½ºÅÍ"  # 몬스터 as the client folder stores it
DATAINFO = CLIENT / "data" / "luafiles514" / "lua files" / "datainfo"

# 1. Prontera Seasons
with tempfile.TemporaryDirectory() as tmp:
    subprocess.run([sys.executable, "-I", str(HERE / "seasons-names.py"),
                    str(PICKS / "prontera-seasons" / "prontera_seasons_v3-3.zip"), SEASON, tmp, GRFLIST], check=True)
    n = 0
    for f in pathlib.Path(tmp, "data").rglob("*"):
        if f.is_file():
            dst = CLIENT / "data" / f.relative_to(pathlib.Path(tmp, "data"))
            dst.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(f, dst)
            n += 1
    print(f"Prontera Seasons ({SEASON}): {n} files")

# 2. + 3. sprites and ids
mobs = [l.split("\t") for l in (HERE / "porings.tsv").read_text().splitlines() if l and not l.startswith("#")]
looks = [l.split("\t") for l in (HERE / "npc_looks.tsv").read_text().splitlines() if l and not l.startswith("#")]
src_mob = PICKS / "poring-collection" / "Poring Family_x" / "Poring Family"
src_npc = PICKS / "npc-100" / "100NPCs_x" / "100NPCs"
d_mob = CLIENT / "data" / "sprite" / MONSTER
d_npc = CLIENT / "data" / "sprite" / "npc"
d_mob.mkdir(parents=True, exist_ok=True)
d_npc.mkdir(parents=True, exist_ok=True)
for row in mobs:
    mid, sprite = row[0], row[1]
    for ext in ("spr", "act"):
        shutil.copyfile(src_mob / f"{sprite}.{ext}", d_mob / f"hlro_{sprite.lower()}.{ext}")
for nid, name in looks:
    folder = src_npc / "MonsterMobs" if name == "sonic" else src_npc
    for ext in ("spr", "act"):
        found = [p for p in folder.iterdir() if p.name.lower() == f"{name}.{ext}"]
        if len(found) != 1:
            raise SystemExit(f"missing sprite {name}.{ext}")
        shutil.copyfile(found[0], d_npc / f"hlro_{name}.{ext}")
print(f"sprites: {len(mobs)} Porings, {len(looks)} NPC looks")

ident = (DATAINFO / "npcidentity.lub").read_text(encoding="latin-1")
names = (DATAINFO / "jobname.lub").read_text(encoding="latin-1")
if "JT_HLRO_TOMB_" not in ident:
    raise SystemExit("run content/world-round1/build-client.py first")
add_i, add_n = [], []
for kind, rows in (("MOB", [(r[0], r[1].lower()) for r in mobs]), ("NPC", looks)):
    for nid, name in rows:
        key = f"JT_HLRO_{kind}_{name.upper()}"
        if f"\t{key} = " in ident:
            continue
        if f"= {nid},\n" in ident:
            raise SystemExit(f"id {nid} already used")
        add_i.append(f"\t{key} = {nid},\n")
        add_n.append(f'\t[jobtbl.{key}] = "hlro_{name.lower()}",\n')
(DATAINFO / "npcidentity.lub").write_text(ident[:-2] + "".join(add_i) + "}\n", encoding="latin-1")
(DATAINFO / "jobname.lub").write_text(names[:-2] + "".join(add_n) + "}\n", encoding="latin-1")
print(f"npcidentity.lub + jobname.lub: {len(add_i)} ids added")
