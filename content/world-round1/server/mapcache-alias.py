"""Puts copies of existing maps under new names into db/import/map_cache.dat, keeping the
other maps already there (e.g. a season's prontera from content/world-round3).
usage: mapcache-alias.py <rathena dir> <new>=<existing> ...   (e.g. 1@mtower=1@tower)"""
import struct, sys
ra = sys.argv[1]


def read_cache(path):
    d = open(path, "rb").read()
    _, count = struct.unpack_from("<IH", d, 0)
    p, out = 8, []
    for _ in range(count):
        name, xs, ys, ln = struct.unpack_from("<12shhi", d, p)
        out.append((name.split(b"\0")[0].decode(), d[p + 12:p + 20 + ln]))
        p += 20 + ln
    return out


maps = {}
for cache in ("db/map_cache.dat", "db/re/map_cache.dat"):
    for name, rest in read_cache(f"{ra}/{cache}"):
        maps[name] = rest
pairs = [a.split("=") for a in sys.argv[2:]]
new_names = {n for n, _ in pairs}
entries = [(n, r) for n, r in read_cache(f"{ra}/db/import/map_cache.dat") if n not in new_names]
for new, old in pairs:
    assert len(new) < 12 and old in maps, (new, old)
    entries.append((new, maps[old]))
body = b"".join(n.encode().ljust(12, b"\0") + r for n, r in entries)
out = struct.pack("<IHH", 8 + len(body), len(entries), 0) + body
open(f"{ra}/db/import/map_cache.dat", "wb").write(out)
print(f"db/import/map_cache.dat: {len(entries)} map(s), {len(out)} bytes")
