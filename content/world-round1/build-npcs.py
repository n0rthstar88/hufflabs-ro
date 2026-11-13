#!/usr/bin/env python3
"""Builds HuffLabs versions of the round-1 community NPC scripts.

usage: build-npcs.py <picks dir> <out dir>

<picks dir> is the quarantine folder of rathena.org downloads
(~/Games/Ragnarok/custom-downloads/picks). Every change is an exact
replacement that must match once, so a changed upstream file fails loudly.
The scripts stay their authors' work; see README.md for credits.
"""
import pathlib
import sys

PICKS = pathlib.Path(sys.argv[1])
OUT = pathlib.Path(sys.argv[2])
OUT.mkdir(parents=True, exist_ok=True)

COIN = 2600000  # HuffLabs Coin (content/coin-shop)


def read(rel):
    return (PICKS / rel).read_bytes().decode("utf-8").replace("\r\n", "\n").replace("\r", "\n")


def sub(text, old, new, count=1):
    n = text.count(old)
    if n != count:
        raise SystemExit(f"expected {count} of {old[:60]!r}, found {n}")
    return text.replace(old, new)


def modernize(text):
    """Old commands this rAthena no longer has: strmobinfo -> getmonsterinfo, and
    global functions called by bare name -> callfunc."""
    import re
    text = re.sub(r'strmobinfo\(\s*1,(getd\(Ac\(\) \+ "ID_" \+ \.@(?:i|s1)\))\)', r"getmonsterinfo(\1,MOB_NAME)", text)
    text = text.replace("strmobinfo(4,2411)", "getmonsterinfo(2411,MOB_MAXHP)")
    text = text.replace("F_InsertComma(", 'callfunc("F_InsertComma",')
    if "strmobinfo" in text:
        raise SystemExit("strmobinfo left over")
    return text


# New faces from the 100 NPCs pack (ids from content/world-round3/npc_looks.tsv; the client
# files come from content/world-round3/build-client.py).
FACES = [
    ("adv_stylist.txt", "\tAdv. Stylist#main\t878,", "\tAdv. Stylist#main\t19225,"),        # dressedgirl
    ("woe_controller.txt", "\tWOE Information\t835,", "\tWOE Information\t19285,"),       # swordknight
    ("dress_room.txt", "\tDress Room#sc\t2_M_DYEINGER,", "\tDress Room#sc\t19210,"),       # cassandra
    ("sader_enchant.txt", "\tEnchanter#sader\t998,", "\tEnchanter#sader\t19291,"),         # windypriest
    ("hunting_quest.txt", "\tHunting Quest\t667,", "\tHunting Quest\t19204,"),             # army
    ("mini_endless_tower.txt", "\tMini Endless Tower\t406,", "\tMini Endless Tower\t19200,"),  # abyssknight
    ("costume_enchant.txt", "\tEnchant Master\t810,", "\tEnchant Master\t19246,"),          # maid
    ("build_manager.txt", "\tBuild Manager\t757,", "\tBuild Manager\t19263,"),               # reader
    ("item_mall.txt", "\tMall Warper\t477,", "\tMall Warper\t19206,"),                       # assistant
]


def write(name, text, source):
    text = modernize(text)
    for fname, old, new in FACES:
        if fname == name:
            text = sub(text, old, new)
    head = f"//===== HuffLabs RO =========================================\n//= Built by content/world-round1/build-npcs.py from\n//= {source}\n//==========================================================\n"
    (OUT / name).write_text(head + text.rstrip("\n") + "\n")
    print(f"{name}: written")


# Punching Bag (Secretdataz): five DPS dummies in Izlude.
t = read("punching-bag/PunchingBag.txt")
t = sub(t, "prontera,224,79,0\tscript\tPunchingBag#0", "izlude,128,128,0\tscript\tPunchingBag#0")
t = sub(t, "prontera,216,79,0\tduplicate", "izlude,125,125,0\tduplicate")
t = sub(t, "prontera,224,70,0\tduplicate", "izlude,125,128,0\tduplicate")
t = sub(t, "prontera,216,58,0\tduplicate", "izlude,125,131,0\tduplicate")
t = sub(t, "prontera,224,58,0\tduplicate", "izlude,128,125,0\tduplicate")
write("punching_bag.txt", t, "Punching Bag 0.2 by Secretdataz")

# Build Manager (Emistry): saves and loads stat builds, in Juno.
# Fix: loading reset all skills and gave back only the points that were unspent
# when the build was saved, so loading wiped your skills. Skills are left alone now.
t = read("build-manager/Build Manager 4.1.txt")
t = sub(t, "prt_in,167,171,3\tscript\tSample\t757,{", "yuno,162,56,4\tscript\tBuild Manager\t757,{")
t = sub(t, "\t\t\tresetskill;\n", "")
t = sub(t, "\t\t\tSkillPoint = .@skill_point[.@index];\n", "")
write("build_manager.txt", t, "Build Manager 4.1 by Emistry (skill reset removed)")

# Advanced Stylist: one stylist in Prontera (the three duplicates all stood on one cell).
t = read("advanced-stylist/Advance Stylist 4.4.txt")
t = sub(t, "prontera,155,181,5\tscript\tAdv. Stylist#main", "prontera,160,182,5\tscript\tAdv. Stylist#main")
for i in (1, 2, 3):
    t = sub(t, f"prontera,115,181,5\tduplicate(Adv. Stylist#main)\tAdv. Stylist#{i}\t878\n", "")
# Extra dyes (content/world-round4): per-character colour limit and fix-up.
t = sub(t, "\t@style_value = getlook(.look_type[@style]);\n", "\t@style_value = getlook(.look_type[@style]);\n\t.@max = callfunc(\"F_HLRO_DyeMax\", @style, .max_style[@style]); // HuffLabs dyes\n")
for old_max in ("if (.@next > .max_style[@style])", "if (.@prev < .min_style[@style]) .@prev = .max_style[@style];",
                "rand(.min_style[@style], .max_style[@style])", "\" ~ \"+.max_style[@style]+\".\"", "input .@style_number,.min_style[@style],.max_style[@style];"):
    t = sub(t, old_max, old_max.replace(".max_style[@style]", ".@max"))
t = sub(t, "\t\t\tsetlook .look_type[@style],.@style_number;\n", "\t\t\tsetlook .look_type[@style],.@style_number;\n\t\t\tcallfunc \"F_HLRO_DyeFix\";\n")
write("adv_stylist.txt", t, "Advance Stylist 4.4")

# Sader's Enchantment: any gear (not only the sample novice items), no breaking, in Geffen.
t = read("saders-enchant/sader_s_enchant.txt")
t = sub(t, "prontera,157,176,6\tscript\tsader enchant\t998,{", "geffen,124,61,4\tscript\tEnchanter#sader\t998,{")
for kind in ("Weapons", "Armors", "Shields", "Germents", "Shoses", "Accessarys", "Uppers", "Middels", "Lowers"):
    t = sub(t, f"\t.specific_{kind} = true;", f"\t.specific_{kind} = false;")
t = sub(t, "\tsetarray .s_level_required,0,175;", "\tsetarray .s_level_required,1,500;")
t = sub(t, "\t.brack_chance = 50;", "\t.brack_chance = 0;")
t = sub(t, "\t.enable_the_shop = true;", "\t.enable_the_shop = false;")
write("sader_enchant.txt", t, "Sader's Enchantment 3.1 by Sader1992 (any gear, never breaks)")

# Dress Room (Secret, MIT): unchanged, Morroc.
write("dress_room.txt", read("dress-changer/DressRoom.txt"), "Dress Room 1.0 by Secret (MIT)")

# Item Mall: unchanged (the owner chose "everything as-is"). Entrance NPC in Amatsu.
# Six prices sit below what NPCs pay with Overcharge, so buying here and selling anywhere
# made free zeny. Raised so Discount 24% still costs more than Overcharge 24% pays.
import re as _re
t = read("item-mall/mall.txt")
for iid, price in ((2809, 1700000), (2810, 4200000), (579, 600), (610, 10000), (713, 350)):
    t, n = _re.subn(rf"(?<=,){iid}:\d+", f"{iid}:{price}", t)
    assert n, iid
write("item_mall.txt", t, "Item Mall (mall.txt), 5 prices raised to stop a buy-low/sell-high loop")

# WoE controller (Euphy, updated by BeWan): no schedule until a GM sets one in its menu.
# Castle reward: 10 HuffLabs Coins instead of a Recruit Siege Supply Box.
t = read("woe-controller/BeWan_WOE_Controller.txt")
t = sub(t, "prontera,149,193,4\tscript\tWOE Information", "prontera,144,198,4\tscript\tWOE Information")
t = sub(t, "\tsetarray .reward_id[0],14001;\n\tsetarray .reward_amount[0],1;", f"\tsetarray .reward_id[0],{COIN};\n\tsetarray .reward_amount[0],10;")
write("woe_controller.txt", t, "Euphy's WOE Controller 1.9, BeWan update 1.4")

# Mini Endless Tower (Sader1992): Alberta; map renamed to lowercase 1@mtower.
# Reward per clear: 20 coins + 10% of the next base/job level.
t = read("mini-endless-tower/Saders_MiniEndlessTower.txt")
t = sub(t, "1@Mtower", "1@mtower", t.count("1@Mtower"))
t = sub(t, "prontera,155,186,3\tscript\tMini Endless Tower\t406,{", "alberta,191,145,4\tscript\tMini Endless Tower\t406,{")
t = sub(t, "\t//you can add rewards here !! \n", f"\t// HuffLabs reward per clear\n\tgetitem {COIN},20;\n\tgetexp readparam(NextBaseExp) / 10, readparam(NextJobExp) / 10;\n")
write("mini_endless_tower.txt", t, "saders Mini Endless Tower 1.0 by Sader1992")

# Hunting Quest (Sader1992): Payon. Points buy HuffLabs Coins (10 points = 1 coin).
# Five tiers by player level, with monsters that actually spawn at those levels.
t = read("hunting-quest/Hunting_Quest.txt")
t = sub(t, "-\tpointshop\tHunting_Quest_Shop\t-1,#HUNT_Q_POINTS,512:1;", f"-\tpointshop\tHunting_Quest_Shop\t-1,#HUNT_Q_POINTS,{COIN}:10;")
t = sub(t, "prontera,155,176,4\tscript\tHunting Quest\t667,{", "payon,165,225,4\tscript\tHunting Quest\t667,{")
a = t.index("\tAddCata(\"Easy\"")
b = t.index("end;\n}", a)
tiers = '''\tAddCata("Rookie (1-60)",.Rookie,1,60,4,2000,1000,30);
\tAddCata("Adventurer (50-130)",.Adventurer,50,130,6,50000,20000,60);
\tAddCata("Veteran (100-200)",.Veteran,100,200,8,500000,200000,100);
\tAddCata("Elite (150-300)",.Elite,150,300,10,3000000,1000000,150);
\tAddCata("Master (190-500)",.Master,190,500,12,10000000,3000000,250);

\t// Rookie: Fabre, Roda Frog, Poporing, Elder Willow, Coco
\tAddMob(.Rookie,1007,40,60); AddMob(.Rookie,1012,40,60); AddMob(.Rookie,1031,40,60); AddMob(.Rookie,1033,40,60); AddMob(.Rookie,1104,40,60);
\t// Adventurer: Argiope, Metaling, Geographer, Grand Peco, Mantis, Harpy
\tAddMob(.Adventurer,1099,60,90); AddMob(.Adventurer,1613,60,90); AddMob(.Adventurer,1368,60,90); AddMob(.Adventurer,1369,60,90); AddMob(.Adventurer,1139,60,90); AddMob(.Adventurer,1376,60,90);
\t// Veteran: Drosera, Hill Wind, Raydric, Zombie Slaughter, Muscipular, Gargoyle
\tAddMob(.Veteran,1781,80,120); AddMob(.Veteran,1680,80,120); AddMob(.Veteran,1163,80,120); AddMob(.Veteran,1864,80,120); AddMob(.Veteran,1780,80,120); AddMob(.Veteran,1253,80,120);
\t// Elite: Broken Cleaning Robot, Broken Beta Guards, Big Bell, Mutant Dolor, Explorer Robot Turbo, Broken Warehouse Manager
\tAddMob(.Elite,20641,100,150); AddMob(.Elite,20639,100,150); AddMob(.Elite,2917,100,150); AddMob(.Elite,20362,100,150); AddMob(.Elite,3249,100,150); AddMob(.Elite,20637,100,150);
\t// Master: Spell Addicted Sanare / Plaga, Sharp Spell, Powerful Spell
\tAddMob(.Master,20692,150,200); AddMob(.Master,20691,150,200); AddMob(.Master,20694,150,200); AddMob(.Master,20693,150,200);

'''
t = t[:a] + tiers + t[b:]
write("hunting_quest.txt", t, "saders hunting quest 5.2 by Sader1992 (HuffLabs tiers, points buy coins)")

# Costume enchant (Dimas_Shotta93): Al De Baran, costs 10 HuffLabs Coins instead of 10 Silvervine Fruit.
t = read("costume-enchant/super_enchant.txt")
t = sub(t, "prt_in,36,95,2\tscript\tEnchant Master\t810,{", "aldebaran,142,123,4\tscript\tEnchant Master\t810,{")
t = sub(t, "countitem(6417)", f"countitem({COIN})", t.count("countitem(6417)"))
t = sub(t, "delitem 6417,10", f"delitem {COIN},10", t.count("delitem 6417,10"))
t = t.replace("getitemname(6417)", f"getitemname({COIN})")
t = t.replace("silvervine fruit", "HuffLabs Coins").replace("Silvervine Fruit", "HuffLabs Coins")
if "6417" in t:
    raise SystemExit("super_enchant still mentions 6417")
write("costume_enchant.txt", t, "super_enchant 0.1 by Dimas_Shotta93 (costs coins)")
