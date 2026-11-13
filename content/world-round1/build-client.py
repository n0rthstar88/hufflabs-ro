#!/usr/bin/env python3
"""Adds the round-1 client files to a HuffLabs RO client folder.

usage: build-client.py <picks dir> <client dir> <grftool> <scratch dir>

- Mob Element View icons -> data/texture/<UI>/group/group_51..60.bmp
- MVP tomb sprites (DynamicMVPTomb's new_tomb.grf) -> data/sprite/npc/<mob>n.spr/.act
- npcidentity.lub + jobname.lub as plain Lua (dumped from data.grf) with the tomb ids 19100-19190
- resnametable.txt (from data.grf) + the 1@mtower -> 1@tower alias for Mini Endless Tower

Community art is copied from the quarantine folder, never committed to the repo.
Needs lua5.1 (the dump runs Gravity's own bytecode in an empty Lua environment).
"""
import pathlib
import re
import shutil
import subprocess
import sys

PICKS, CLIENT, GRFTOOL, SCRATCH = (pathlib.Path(a) for a in sys.argv[1:5])
HERE = pathlib.Path(__file__).resolve().parent
UI = "À¯ÀúÀÎÅÍÆäÀÌ½º"  # 유저인터페이스 as the client stores it
DATAINFO = CLIENT / "data" / "luafiles514" / "lua files" / "datainfo"
SCRATCH.mkdir(parents=True, exist_ok=True)


def grf_extract(name, out):
    subprocess.run([str(GRFTOOL), "extract", str(CLIENT / "data.grf"), name, str(out)], check=True)


# 1. element icons
src = next((PICKS / "mob-element-view").glob("data (5)_x/data/texture/*/group"))
dst = CLIENT / "data" / "texture" / UI / "group"
dst.mkdir(parents=True, exist_ok=True)
for n in range(51, 61):
    shutil.copyfile(src / f"group_{n}.bmp", dst / f"group_{n}.bmp")
print("element icons: 10 copied")

# 2. tomb sprites
tomb_grf = PICKS / "mvp-tombstone" / "DynamicTomb_x" / "new_tomb.grf"
names = []
for line in subprocess.run([str(GRFTOOL), "list", str(tomb_grf)], check=True, capture_output=True, text=True).stdout.splitlines():
    name = line.split("\t", 1)[1]
    m = re.fullmatch(r"data\\sprite\\npc\\(\d{4}n)\.(spr|act)", name)
    if not m:
        raise SystemExit(f"unexpected file in new_tomb.grf: {name!r}")
    names.append((name, m.group(1) + "." + m.group(2)))
spr = CLIENT / "data" / "sprite" / "npc"
spr.mkdir(parents=True, exist_ok=True)
for name, base in names:
    subprocess.run([str(GRFTOOL), "extract", str(tomb_grf), name, str(spr / base)], check=True)
print(f"tomb sprites: {len(names)} files")

# 3. npc tables as plain Lua, plus the tomb ids
tomb_ids = [(m.group(1), int(m.group(2))) for m in re.finditer(r"\{\s*(\d{4}),\s*(19\d{3})\s*\}", (HERE / "rathena.patch").read_text())]
assert len(tomb_ids) == 91, len(tomb_ids)
sprites = {b[:-4] for _, b in names}
for mob, _ in tomb_ids:
    assert f"{mob}n" in sprites, mob
lub = SCRATCH / "lub"
lub.mkdir(exist_ok=True)
for f in ("npcidentity", "jobname"):
    grf_extract(f"data\\luafiles514\\lua files\\datainfo\\{f}.lub", lub / f"{f}.lub")
    subprocess.run([sys.executable, "-I", str(HERE / "lub32to64.py"), str(lub / f"{f}.lub"), str(lub / f"{f}.64.lub")], check=True)
text = lub / "text"
text.mkdir(exist_ok=True)
subprocess.run(["lua5.1", str(HERE / "lubdump.lua"), str(lub / "npcidentity.64.lub"), str(lub / "jobname.64.lub"), str(text)], check=True)
ident = (text / "npcidentity.lub").read_text(encoding="latin-1")
names_lua = (text / "jobname.lub").read_text(encoding="latin-1")
assert ident.endswith("}\n") and names_lua.endswith("}\n")
for mob, nid in tomb_ids:
    assert f"= {nid}," not in ident, f"id {nid} already used"
ident = ident[:-2] + "".join(f"\tJT_HLRO_TOMB_{mob} = {nid},\n" for mob, nid in tomb_ids) + "}\n"
names_lua = names_lua[:-2] + "".join(f'\t[jobtbl.JT_HLRO_TOMB_{mob}] = "{mob}n",\n' for mob, _ in tomb_ids) + "}\n"
DATAINFO.mkdir(parents=True, exist_ok=True)
(DATAINFO / "npcidentity.lub").write_text(ident, encoding="latin-1")
(DATAINFO / "jobname.lub").write_text(names_lua, encoding="latin-1")
print(f"npcidentity.lub + jobname.lub: plain Lua, {len(tomb_ids)} tomb ids added")

# 4. map alias for the Mini Endless Tower instance
grf_extract("data\\resnametable.txt", lub / "resnametable.txt")
res = (lub / "resnametable.txt").read_bytes()
if b"1@mtower" not in res:
    if not res.endswith(b"\n"):
        res += b"\r\n"
    ui = UI.encode("latin-1")
    res += (b"1@mtower.gnd#1@tower.gnd#\r\n1@mtower.gat#1@tower.gat#\r\n1@mtower.rsw#1@tower.rsw#\r\n"
            + ui + b"\\map\\1@mtower.bmp#" + ui + b"\\map\\1@tower.bmp#\r\n")
(CLIENT / "data" / "resnametable.txt").write_bytes(res)
print("resnametable.txt: 1@mtower alias added")

# 5. Super Saiyan Aura costume (rathena.org free headgear; Dragon Ball fan art, private server only).
#    Headgear look 2935 = sprite "_saiyan_aura"; server item 2600100 is in content/coin-shop.
AURA_VIEW = 2935
aura = PICKS.parent / "ssj" / "Super Saiyan Aura"
ACC = "¾Ç¼¼»ç¸®"  # 악세사리
for sex in ("³²", "¿©"):  # 남, 여
    d = CLIENT / "data" / "sprite" / ACC / sex
    d.mkdir(parents=True, exist_ok=True)
    for ext in ("spr", "act"):
        shutil.copyfile(aura / "sprite" / ACC / sex / f"{sex}_saiyan_aura.{ext}", d / f"{sex}_saiyan_aura.{ext}")
for kind in ("item", "Collection"):
    d = CLIENT / "data" / "texture" / UI / kind.lower()
    d.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(aura / "texture" / UI / kind / "saiyan_aura.bmp", d / "saiyan_aura.bmp")
for f in ("accessoryid", "accname"):
    grf_extract(f"data\\luafiles514\\lua files\\datainfo\\{f}.lub", lub / f"{f}.lub")
    subprocess.run([sys.executable, "-I", str(HERE / "lub32to64.py"), str(lub / f"{f}.lub"), str(lub / f"{f}.64.lub")], check=True)
subprocess.run(["lua5.1", str(HERE / "lubdump.lua"), str(lub / "accessoryid.64.lub"), str(lub / "accname.64.lub"), str(text),
                "ACCESSORY_IDs", "AccNameTable", "accessoryid.lub", "accname.lub"], check=True)
ids = (text / "accessoryid.lub").read_text(encoding="latin-1")
accn = (text / "accname.lub").read_text(encoding="latin-1")
assert f"= {AURA_VIEW}," not in ids, "view id taken"
ids = ids[:-2] + f"\tACCESSORY_HLRO_SAIYAN_AURA = {AURA_VIEW},\n}}\n"
accn = accn[:-2] + '\t[ACCESSORY_IDs.ACCESSORY_HLRO_SAIYAN_AURA] = "_saiyan_aura",\n}\n'
(DATAINFO / "accessoryid.lub").write_text(ids, encoding="latin-1")
(DATAINFO / "accname.lub").write_text(accn, encoding="latin-1")
print(f"Super Saiyan Aura: sprites, icons, headgear look {AURA_VIEW}")
