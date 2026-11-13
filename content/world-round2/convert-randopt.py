#!/usr/bin/env python3
"""Converts the 2017 "Random Option drops" download (item_randomopt_group.txt + mob_drop.txt,
txt format) to this rAthena's YAML.

usage: convert-randopt.py <rathena dir> <group txt> <mob_drop txt> <out dir>
writes: <out>/item_randomopt_group.yml  (import, groups ADO_*)
        <out>/mob_db_randopt.yml        (imported from db/import/mob_db.yml's Footer)

The old group file lists every combination (one weighted row = up to 3 options). The YAML
format can't hold combinations, so each group becomes: slot 1 = one guaranteed option, picked
with the same weights as the old first column; plus up to two random extra options, as likely
as the old rows with 2 or 3 options. Values become min..max ranges of the old values.

Old mob_drop.txt gave options to drops a monster already has (rate overwritten), or added the
drop. Here an existing drop keeps its official rate and gets the option group (by Index); a
missing one is appended with the old rate.
"""
import collections
import re
import sys

RA, GROUPS, DROPS, OUT = sys.argv[1:5]
MAX_DROPS = 10


def yaml_ids(paths, key_re):
    """Id -> (AegisName) for item_db-like files."""
    out, cur = {}, None
    for p in paths:
        try:
            f = open(p, encoding="utf-8", errors="replace")
        except FileNotFoundError:
            continue
        for l in f:
            m = re.match(r"  - Id: (\d+)", l)
            if m:
                cur = int(m.group(1))
                continue
            m = re.match(key_re, l)
            if m and cur is not None and cur not in out:
                out[cur] = m.group(1).strip()
    return out


items = yaml_ids([f"{RA}/db/re/item_db_equip.yml", f"{RA}/db/re/item_db_usable.yml", f"{RA}/db/re/item_db_etc.yml", f"{RA}/db/import/item_db.yml"], r"    AegisName: (\S+)")
options = set()
for p in (f"{RA}/db/re/item_randomopt_db.yml", f"{RA}/db/import/item_randomopt_db.yml"):
    try:
        options |= set(re.findall(r"^\s+Option: (\S+)", open(p).read(), re.M))
    except FileNotFoundError:
        pass

# current drops per monster, in order (official file, then our import changes)
mob_drops, mob_names = {}, {}
cur, in_drops = None, False
for l in open(f"{RA}/db/re/mob_db.yml", encoding="utf-8", errors="replace"):
    m = re.match(r"  - Id: (\d+)", l)
    if m:
        cur, in_drops = int(m.group(1)), False
        mob_drops[cur] = []
        continue
    if cur is None:
        continue
    m = re.match(r"    AegisName: (\S+)", l)
    if m:
        mob_names[cur] = m.group(1)
    if re.match(r"    Drops:", l):
        in_drops = True
        continue
    if re.match(r"    \S", l):
        in_drops = False
    m = re.match(r"      - Item: (\S+)", l)
    if m and in_drops:
        mob_drops[cur].append(m.group(1))
imp = open(f"{RA}/db/import/mob_db.yml").read()
if re.search(r"^\s+Drops:", imp, re.M):
    raise SystemExit("db/import/mob_db.yml already changes drops; teach this script to merge them first")

# ---- groups
rows = collections.defaultdict(list)
for l in open(GROUPS, encoding="latin-1"):
    l = l.split("//")[0].strip()
    if not l:
        continue
    f = [x.strip() for x in l.split(",")]
    opts = []
    for i in range(2, len(f) - 2, 3):
        name = re.sub(r"^RDMOPT_", "", f[i])
        opts.append((name, int(f[i + 1]), int(f[i + 2])))
    rows[f[0]].append((int(f[1]), opts))

used = set()
drops = []
for l in open(DROPS, encoding="latin-1"):
    l = l.split("//")[0].strip()
    if not l:
        continue
    f = [x.strip() for x in l.split(",")]
    drops.append((int(f[0]), int(f[1]), int(f[2]), f[3] if len(f) > 3 else ""))
    used.add(f[3] if len(f) > 3 else "")

problems = collections.Counter()
yml = ["# HuffLabs RO import (content/world-round2): Random Option drop groups, converted from the",
       "# 2017 rathena.org \"Random Option drops\" download by convert-randopt.py.",
       "Header:", "  Type: RANDOM_OPTION_GROUP", "  Version: 1", "", "Body:"]
gid = 1000  # official groups use ids up to ~173
gnames = {}


def pool(entries):
    """entries: list of (name, value, param, weight) -> {(name, param): [min, max, weight]}"""
    p = {}
    for name, val, param, w in entries:
        if name not in options:
            problems[f"unknown option {name}"] += 1
            continue
        k = (name, param)
        if k not in p:
            p[k] = [val, val, 0]
        p[k][0] = min(p[k][0], val)
        p[k][1] = max(p[k][1], val)
        p[k][2] += w
    return p


def emit(p, indent, scale):
    out = []
    for (name, param), (lo, hi, w) in sorted(p.items()):
        chance = max(1, min(10000, round(scale * w)))
        out += [f"{indent}- Option: {name}", f"{indent}  MinValue: {lo}", f"{indent}  MaxValue: {hi}"]
        if param:
            out.append(f"{indent}  Param: {param}")
        out.append(f"{indent}  Chance: {chance}")
    return out


for old in sorted(used):
    if old not in rows:
        problems[f"group {old} missing"] += 1
        continue
    rs = rows[old]
    total = sum(r for r, _ in rs)
    first = pool([(o[0][0], o[0][1], o[0][2], r) for r, o in rs if o])
    extra = pool([(n, v, p, r) for r, o in rs for (n, v, p) in o[1:]])
    p2 = sum(r for r, o in rs if len(o) >= 2) / total
    p3 = sum(r for r, o in rs if len(o) >= 3) / total
    name = "ADO_" + re.sub(r"^RDMOPTG_", "", old).upper()
    gnames[old] = name
    yml += [f"  - Id: {gid}", f"    Group: {name}", "    Slots:", "      - Slot: 1", "        Options:"]
    yml += emit(first, "          ", 10000 / max(w for _, _, w in first.values()))
    if extra:
        a = (p2 + p3) / 2  # two tries; expected extras = p2 + p3
        wsum = sum(w for _, _, w in extra.values())
        yml += ["    MaxRandom: 3", "    Random:"]
        yml += emit(extra, "      ", a * 10000 * len(extra) / wsum)
    gid += 1
open(f"{OUT}/item_randomopt_group.yml", "w").write("\n".join(yml) + "\n")

# ---- drops
by_mob = collections.defaultdict(list)
for mob, item, rate, grp in drops:
    if mob not in mob_drops:
        problems["unknown monster"] += 1
        continue
    if item not in items:
        problems["unknown item"] += 1
        continue
    if grp not in gnames:
        problems["drop without a known group"] += 1
        continue
    by_mob[mob].append((items[item], rate, gnames[grp]))

dy = ["# HuffLabs RO (content/world-round2): random options on monster drops.",
      "# Loaded through the Footer of db/import/mob_db.yml. Built by convert-randopt.py.",
      "Header:", "  Type: MOB_DB", "  Version: 5", "", "Body:"]
changed = appended = 0
for mob in sorted(by_mob):
    current = list(mob_drops[mob])
    entries = []
    for aegis, rate, grp in by_mob[mob]:
        if aegis in current:
            entries.append((current.index(aegis), aegis, None, grp))
            changed += 1
        elif len(current) < MAX_DROPS:
            current.append(aegis)
            entries.append((len(current) - 1, aegis, rate, grp))
            appended += 1
        else:
            problems["drop list full"] += 1
    if not entries:
        continue
    dy += [f"  - Id: {mob}", f"    AegisName: {mob_names[mob]}", "    Drops:"]
    for idx, aegis, rate, grp in sorted(entries):
        dy += [f"      - Index: {idx}", f"        Item: {aegis}"]
        if rate is None:  # keep the official rate: read it back from the official entry
            rate = official_rate = None
        dy.append(f"        Rate: @@RATE:{mob}:{idx}@@" if rate is None else f"        Rate: {rate}")
        dy.append(f"        RandomOptionGroup: {grp}")
text = "\n".join(dy) + "\n"

# fill in official rates for overwritten drops
rates = {}
cur, in_drops, idx = None, False, -1
for l in open(f"{RA}/db/re/mob_db.yml", encoding="utf-8", errors="replace"):
    m = re.match(r"  - Id: (\d+)", l)
    if m:
        cur, in_drops, idx = int(m.group(1)), False, -1
        continue
    if re.match(r"    Drops:", l):
        in_drops = True
        continue
    if re.match(r"    \S", l):
        in_drops = False
    if in_drops and re.match(r"      - Item: ", l):
        idx += 1
    m = re.match(r"        Rate: (\d+)", l)
    if m and in_drops and cur is not None:
        rates[(cur, idx)] = m.group(1)
text = re.sub(r"@@RATE:(\d+):(\d+)@@", lambda m: rates[(int(m.group(1)), int(m.group(2)))], text)
open(f"{OUT}/mob_db_randopt.yml", "w").write(text)

print(f"groups: {len(gnames)}; drops: {changed} existing drops got options, {appended} added; monsters: {len(by_mob)}")
for k, v in problems.most_common():
    print(f"  skipped: {k} x{v}")
