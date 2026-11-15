#!/usr/bin/env python3
"""Builds the Prontera Seasons file sets the publisher signs for every season.

usage: build-season-sets.py <picks dir> <client dir> <grf file list> <out dir>
  <grf file list>: `grftool list data.grf` output
  writes <out dir>/<season>/data/... for spring, summer, fall and winter

hufflabs-ro-publish --seasons <out dir> then signs one manifest per season, and the server
switches between them on the season's date (server-season-switch.sh), so the publisher's
PC doesn't need to be on. File names reuse the spelling the client folder already has
(Windows names are case-insensitive; see build-client.py).
"""
import pathlib
import shutil
import subprocess
import sys
import tempfile

PICKS, CLIENT, GRFLIST, OUT = sys.argv[1], pathlib.Path(sys.argv[2]), sys.argv[3], pathlib.Path(sys.argv[4])
HERE = pathlib.Path(__file__).resolve().parent
ZIP = pathlib.Path(PICKS) / "prontera-seasons" / "prontera_seasons_v3-3.zip"

for season in ("spring", "summer", "fall", "winter"):
    dest = OUT / season
    if dest.exists():
        shutil.rmtree(dest)
    with tempfile.TemporaryDirectory() as tmp:
        subprocess.run([sys.executable, "-I", str(HERE / "seasons-names.py"), str(ZIP), season, tmp, GRFLIST],
                       check=True, stdout=subprocess.DEVNULL)
        n = 0
        for f in sorted(pathlib.Path(tmp, "data").rglob("*")):
            if not f.is_file():
                continue
            rel = f.relative_to(tmp)
            # reuse the client's spelling for the folder and the file when it has them
            parts, cdir = [], CLIENT
            for i, part in enumerate(rel.parts):
                same = [p for p in cdir.iterdir() if p.name.lower() == part.lower()] if cdir.is_dir() else []
                name = same[0].name if same else part
                parts.append(name)
                cdir = cdir / name
            dst = dest.joinpath(*parts)
            dst.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(f, dst)
            n += 1
    print(f"{season}: {n} files")
