#!/usr/bin/env python3
"""Puts a map built from a .gat file into rAthena's import map cache (db/import/map_cache.dat),
keeping the other maps already there (e.g. the 1@mtower alias).

usage: mapcache-gat.py <rathena dir> <map name> <file.gat>
Cell types are taken from the .gat; cells that are water in the official cache stay water
(rAthena derives water from the .rsw water level, which this map pack doesn't change)."""
import struct, sys, zlib

ra, name, gatfile = sys.argv[1], sys.argv[2], sys.argv[3]


def read_cache(path):
    d = open(path, "rb").read()
    _, n = struct.unpack_from("<IH", d, 0)
    p, out = 8, []
    for _ in range(n):
        nm, xs, ys, ln = struct.unpack_from("<12shhi", d, p)
        out.append((nm.split(b"\0")[0].decode(), xs, ys, d[p + 20:p + 20 + ln]))
        p += 20 + ln
    return out


official = None
for c in ("db/re/map_cache.dat", "db/map_cache.dat"):
    for nm, xs, ys, z in read_cache(f"{ra}/{c}"):
        if nm == name and official is None:
            official = (xs, ys, zlib.decompress(z))
if official is None:
    raise SystemExit(f"{name} is not in the official map cache")

g = open(gatfile, "rb").read()
if g[:4] != b"GRAT":
    raise SystemExit("not a .gat file")
xs, ys = struct.unpack_from("<ii", g, 6)
if (xs, ys) != official[:2]:
    raise SystemExit(f"size {xs}x{ys} differs from the official {official[0]}x{official[1]}")
cells = bytearray(struct.unpack_from("<i", g, 14 + i * 20 + 16)[0] for i in range(xs * ys))
for i, t in enumerate(official[2]):
    if t == 3 and cells[i] == 0:
        cells[i] = 3

entries = [e for e in read_cache(f"{ra}/db/import/map_cache.dat") if e[0] != name]
entries.append((name, xs, ys, zlib.compress(bytes(cells), 9)))
body = b"".join(n.encode().ljust(12, b"\0") + struct.pack("<hhi", x, y, len(z)) + z for n, x, y, z in entries)
open(f"{ra}/db/import/map_cache.dat", "wb").write(struct.pack("<IHH", 8 + len(body), len(entries), 0) + body)
print(f"db/import/map_cache.dat: {name} from {gatfile}; maps: {', '.join(e[0] for e in entries)}")
