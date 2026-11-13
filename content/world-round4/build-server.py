#!/usr/bin/env python3
"""Writes the round-4 server files from headgears.tsv.
usage: build-server.py <out dir>
  <out>/db/import/item_db_headgears.yml   316 stat-less costume headgears (imported via the
                                          Footer of the coin shop's db/import/item_db.yml block)
  <out>/npc/costume_tailors.txt           three coin shops in Alberta (upper / middle / lower)"""
import pathlib, sys
HERE = pathlib.Path(__file__).resolve().parent
OUT = pathlib.Path(sys.argv[1])
(OUT / "db" / "import").mkdir(parents=True, exist_ok=True)
(OUT / "npc").mkdir(parents=True, exist_ok=True)
COIN, PRICE = 2600000, 200
rows = [l.split("\t")[:5] for l in (HERE / "headgears.tsv").read_text().splitlines() if l and not l.startswith("#")]
LOC = {"Top": ["Costume_Head_Top"], "Mid": ["Costume_Head_Mid"], "Low": ["Costume_Head_Low"], "Mid,Low": ["Costume_Head_Mid", "Costume_Head_Low"]}
y = ["# HuffLabs RO (content/world-round4): costume headgears from Mihael's custom pack, no stats.",
     "# Built by build-server.py from headgears.tsv.", "Header:", "  Type: ITEM_DB", "  Version: 3", "", "Body:"]
for iid, view, spr, slots, name in rows:
    y += [f"  - Id: {iid}", f"    AegisName: C_HLRO_{spr.upper().replace('-', '_')}", f"    Name: Costume {name}",
          "    Type: Armor", "    Weight: 0", "    Locations:"]
    y += [f"      {loc}: true" for loc in LOC[slots]]
    y += ["    ArmorLevel: 1", "    EquipLevelMin: 1", f"    View: {view}"]
(OUT / "db" / "import" / "item_db_headgears.yml").write_text("\n".join(y) + "\n")
shops = [("117,62", "Upper", ("Top",)), ("116,59", "Middle", ("Mid", "Mid,Low")), ("116,56", "Lower", ("Low",))]
t = ["//===== HuffLabs RO =========================================",
     "//= Costume Tailors (content/world-round4): Mihael's custom headgears as costumes,",
     f"//= {PRICE} HuffLabs Coins each. Built by build-server.py.",
     "//=========================================================="]
looks = {"Upper": 19246, "Middle": 19225, "Lower": 19210}  # maid, dressedgirl, cassandra (round-3 NPC looks)
for xy, label, keep in shops:
    ids = [r[0] for r in rows if r[3] in keep]
    t.append(f"alberta,{xy},4\titemshop\tCostume Tailor ({label})#hlro\t{looks[label]},{COIN}," + ",".join(f"{i}:{PRICE}" for i in ids))
(OUT / "npc" / "costume_tailors.txt").write_text("\n".join(t) + "\n")
print(f"{len(rows)} headgears; shops: " + ", ".join(f"{l} {sum(1 for r in rows if r[3] in k)}" for _, l, k in shops))
