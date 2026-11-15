#!/usr/bin/env python3
"""Fixes the facing directions of the 100 NPCs pack sprites, in place.

usage: fix-npc-act.py <file.act> [...]

The pack drew each look once, for the south-facing action (0), and filled the other seven
directions from a template whose offsets don't match the picture. Seen from another camera
angle, or placed facing another way, the NPC sinks into the floor (Adv. Stylist) or slides
off sideways (the 338-px-wide sword knight). This copies action 0's layer into actions 1-7,
so every direction shows the one good picture. Only files with a single frame and a single
layer per direction are touched; anything else (e.g. the animated sonic sprite) is left alone.
"""
import struct
import sys


def layer_spans(d):
    """Returns, per action, a list of frames, each a list of (start, end) byte spans of its layers."""
    ver, nact = struct.unpack_from("<HH", d, 2)
    o, acts = 16, []
    for _ in range(nact):
        (nf,) = struct.unpack_from("<I", d, o)
        o += 4
        frames = []
        for _ in range(nf):
            o += 32
            (nl,) = struct.unpack_from("<I", d, o)
            o += 4
            spans = []
            for _ in range(nl):
                s = o
                o += 16  # x, y, sprite, mirror
                if ver >= 0x200:
                    o += 4 + (8 if ver >= 0x204 else 4) + 4 + 4  # colour, scale, rotation, sprite type
                    if ver >= 0x205:
                        o += 8  # width, height
                spans.append((s, o))
            if ver >= 0x200:
                o += 4  # event id
            if ver >= 0x203:
                (na,) = struct.unpack_from("<I", d, o)
                o += 4 + 16 * na
            frames.append(spans)
        acts.append(frames)
    return acts


def fix(path):
    d = bytearray(open(path, "rb").read())
    if d[:2] != b"AC":
        return "not an ACT file"
    acts = layer_spans(d)
    if len(acts) < 8 or any(len(a) != 1 or len(a[0]) != 1 for a in acts[:8]):
        return "skipped (animated or multi-layer)"
    s0, e0 = acts[0][0][0]
    good = bytes(d[s0:e0])
    changed = 0
    for a in acts[1:8]:
        s, e = a[0][0]
        if d[s:e] != good:
            d[s:e] = good
            changed += 1
    if changed:
        open(path, "wb").write(d)
    return f"fixed {changed} directions" if changed else "already fine"


if __name__ == "__main__":
    for p in sys.argv[1:]:
        print(p, fix(p))
