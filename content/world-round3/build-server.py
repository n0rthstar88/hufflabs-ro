#!/usr/bin/env python3
"""Writes the round-3 server files from porings.tsv and npc_looks.tsv.

usage: build-server.py <out dir>
  <out>/db/import/mob_db_porings.yml   Poring Family monsters (imported via the leveling mob_db Footer)
  <out>/npc/poring_family.txt          their field spawns
  <out>/npc/townsfolk.txt              townsfolk using the new NPC looks
"""
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
OUT = pathlib.Path(sys.argv[1])
(OUT / "db" / "import").mkdir(parents=True, exist_ok=True)
(OUT / "npc").mkdir(parents=True, exist_ok=True)

looks = {name: nid for nid, name in (l.split("\t") for l in (HERE / "npc_looks.tsv").read_text().splitlines() if l and not l.startswith("#"))}
mobs = [l.split("\t") for l in (HERE / "porings.tsv").read_text().splitlines() if l and not l.startswith("#")]

y = ["# HuffLabs RO (content/world-round3): Poring Family monsters. Built by build-server.py.",
     "# Loaded through the Footer of db/import/mob_db.yml (content/leveling).",
     "Header:", "  Type: MOB_DB", "  Version: 5", "", "Body:"]
spawns = ["//===== HuffLabs RO =========================================",
          "//= Poring Family spawns (content/world-round3, built by build-server.py)",
          "//= Sprites: Poring Family pack from rathena.org.",
          "//=========================================================="]
for mid, sprite, name, lv, ele, elv, race, size, hp, cls, where in mobs:
    lv, hp = int(lv), int(hp)
    boss = cls == "Boss"
    neko = "NEKO" in sprite
    y += [f"  - Id: {mid}", f"    AegisName: HLRO_{sprite}", f"    Name: {name}", f"    Level: {lv}", f"    Hp: {hp}",
          f"    BaseExp: {30000 if boss else 150 * lv}", f"    JobExp: {10000 if boss else 40 * lv}",
          f"    Attack: {lv * 8}", f"    Attack2: {lv * 10}", f"    Defense: {lv}", f"    MagicDefense: {max(1, lv // 2)}",
          f"    Str: {lv}", f"    Agi: {lv}", f"    Vit: {lv}", f"    Int: {lv // 2}", f"    Dex: {lv + 5}", f"    Luk: {lv}",
          "    AttackRange: 1", "    SkillRange: 10", "    ChaseRange: 12", f"    Size: {size}", f"    Race: {race}",
          f"    Element: {ele}", f"    ElementLevel: {elv}", f"    WalkSpeed: {200 if neko or boss else 400}",
          "    AttackDelay: 1872", "    AttackMotion: 672", "    ClientAttackMotion: 288", "    DamageMotion: 480",
          f"    Ai: {'21' if boss else '02'}"]
    if boss:
        y.append("    Class: Boss")
    y.append("    Drops:")
    drops = ([("HuffLabs_Coin", 10000), ("Old_Blue_Box", 5000), ("Poring_Card", 500), ("Jellopy", 10000)] if boss
             else [("Jellopy", 5000), ("Sticky_Mucus", 1000), ("Apple", 1000), ("HuffLabs_Coin", 200)])
    for item, rate in drops:
        y += [f"      - Item: {item}", f"        Rate: {rate}"]
    for part in where.split(","):
        m, n = part.split(":")
        delay = "1800000,600000" if boss else "5000,0"
        spawns.append(f"{m},0,0\tmonster\t{name}\t{mid},{n},{delay}")
(OUT / "db" / "import" / "mob_db_porings.yml").write_text("\n".join(y) + "\n")
(OUT / "npc" / "poring_family.txt").write_text("\n".join(spawns) + "\n")

# Townsfolk: (map, x, y, dir, look, name, line)
folk = [
    ("prontera", 169, 199, 4, "chef", "Chef Marco", "The fountain water makes the best soup. Don't tell the guards."),
    ("prontera", 140, 175, 6, "rostudent", "Student Lia", "I'm studying for the Sage exam in Juno. The Build Manager there scares me."),
    ("alberta", 175, 135, 4, "pirate", "Captain Brine", "The Mini Endless Tower? I climbed it once. Twenty-five floors of regret."),
    ("amatsu", 209, 94, 4, "samurai", "Wandering Samurai", "The Item Mall up north sells everything. Even honor, I'm told."),
    ("payon", 177, 238, 4, "sniper", "Old Hunter", "Hunting quests pay in coins now. Back in my day it was apples."),
    ("geffen", 109, 70, 4, "redmage", "Apprentice Mage", "The Enchanter never breaks gear. My last teacher did. Often."),
    ("morocc", 150, 101, 4, "dancer", "Desert Dancer", "Try the Dress Room nearby. Everyone looks better as another class."),
    ("izlude", 138, 147, 4, "miniknight", "Little Knight", "Hit the punching bags south of here. They shout your damage!"),
    ("aldebaran", 132, 135, 4, "magicpriest", "Clockwork Priest", "The Enchant Master takes coins for costume enchants. Ten each."),
    ("yuno", 150, 60, 4, "sage", "Lecturer Vane", "Save your stat builds with @bm. Your skills stay safe."),
]
t = ["//===== HuffLabs RO =========================================",
     "//= Townsfolk with the 100 NPCs pack looks (content/world-round3, built by build-server.py)",
     "//=========================================================="]
for i, (m, x, yy, d, look, name, line) in enumerate(folk):
    t += [f"{m},{x},{yy},{d}\tscript\t{name}#hlrofolk{i}\t{looks[look]},{{", f"\tmes \"[{name}]\";", f"\tmes \"{line}\";", "\tclose;", "}", ""]
(OUT / "npc" / "townsfolk.txt").write_text("\n".join(t))
print(f"{len(mobs)} Porings, {len(folk)} townsfolk")
