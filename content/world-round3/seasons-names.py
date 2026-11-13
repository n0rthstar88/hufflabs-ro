#!/usr/bin/env python3
"""Recovers the Korean (cp949) file names in Olrox's Prontera Seasons zip, whose names were
mangled when it was packed. Names are matched against the names the map, ground and model
files reference; the matches teach a per-character table that resolves the rest.

usage: seasons-names.py <zip> <season> <out dir> <grf file list>    (season: fall, winter, spring, summer)
<grf file list> = `grftool list data.grf` output; official names are matched too.
Writes the season's data/ files into <out dir>/data with cp949 names stored the way the
HuffLabs client folder stores them (each byte as a Latin-1 character, like the theme folder)."""
import pathlib, re, sys, zipfile

zpath, season, out, grflist = sys.argv[1], sys.argv[2], pathlib.Path(sys.argv[3]), sys.argv[4]
z = zipfile.ZipFile(zpath)
files = {}
for i in z.infolist():
    raw = i.filename.encode("cp437") if not (i.flag_bits & 0x800) else i.filename.encode("utf-8")
    tag = f"_{season}/data/".encode()
    if tag in raw and not raw.endswith(b"/"):
        files[raw.split(tag)[1]] = i
if not files:
    raise SystemExit(f"no files for season {season}")

refs = {}
for name, info in files.items():
    if name.lower().endswith((b".rsw", b".gnd", b".rsm")):
        for m in re.findall(rb"[\x20-\x7e\x80-\xfe]{2,120}?\.(?:bmp|tga|rsm)", z.read(info), re.I):
            r = m.replace(b"\\", b"/")
            refs.setdefault(r.lower(), r)

official = {}
for line in open(grflist, encoding="utf-8", errors="replace"):
    n = line.rstrip("\n").split("\t", 1)[-1]
    if n.lower().startswith(("data\\model\\", "data\\texture\\")):
        try:
            b = n.encode("cp949").replace(b"\\", b"/")[5:]  # drop "data/"
        except UnicodeEncodeError:
            continue
        official.setdefault(b.lower(), b)


def skel(b):
    return re.sub(rb"[\x80-\xff]", b"?", b.lower())


def chunks(b):
    return re.findall(rb"[\x80-\xff]{2}", b)


table = {}
resolved = {}
skipped = []
for name in files:
    if not re.search(rb"[\x80-\xff]", name):
        resolved[name] = name
        continue
    top, sub = name.split(b"/", 1)
    cands = [refs[k] for k in refs if skel(k) == skel(sub)]
    if len(cands) != 1:  # not referenced by the pack: an official file it replaces?
        cands = [official[k].split(b"/", 1)[1] for k in official if k.startswith(top.lower() + b"/") and skel(k.split(b"/", 1)[1]) == skel(sub)]
    if len(cands) == 1:
        resolved[name] = top + b"/" + cands[0]
        for a, b in zip(chunks(sub), chunks(cands[0])):
            table[a] = b
for name in files:
    if name in resolved:
        continue
    top, sub = name.split(b"/", 1)
    parts = re.split(rb"([\x80-\xff]{2})", sub)
    if all(len(p) != 2 or p in table or not re.match(rb"[\x80-\xff]", p) for p in parts):
        resolved[name] = top + b"/" + b"".join(table.get(p, p) for p in parts)
    else:
        skipped.append(name)

for name, good in resolved.items():
    good.decode("cp949")  # must be valid Korean
    dst = out / "data" / good.decode("latin-1")
    dst.parent.mkdir(parents=True, exist_ok=True)
    dst.write_bytes(z.read(files[name]))
print(f"{season}: {len(resolved)} files written, {len(table)} characters learned")
for n in skipped:
    print("  skipped (name unknown; nothing the map uses references it):", n.decode("cp949", "replace"))
