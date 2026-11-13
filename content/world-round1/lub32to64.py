"""Re-encode a Lua 5.1 bytecode chunk built with 4-byte size_t as 8-byte size_t."""
import struct, sys
src = open(sys.argv[1], "rb").read()
assert src[:12] == b"\x1bLua\x51\x00\x01\x04\x04\x04\x08\x00", src[:12]
p = 12
out = bytearray(b"\x1bLua\x51\x00\x01\x04\x08\x04\x08\x00")
def rd(n):
    global p
    b = src[p:p+n]; p += n; return b
def i32():
    return struct.unpack("<i", rd(4))[0]
def string():
    n = struct.unpack("<I", rd(4))[0]
    out.extend(struct.pack("<Q", n)); out.extend(rd(n))
def copyint():
    v = rd(4); out.extend(v); return struct.unpack("<i", v)[0]
def func():
    string()                      # source
    copyint(); copyint()          # line defined, last line
    out.extend(rd(4))             # nups, numparams, vararg, maxstack
    n = copyint(); out.extend(rd(4 * n))          # code
    n = copyint()                                  # constants
    for _ in range(n):
        t = rd(1); out.extend(t)
        if t == b"\x01": out.extend(rd(1))
        elif t == b"\x03": out.extend(rd(8))
        elif t == b"\x04": string()
        elif t != b"\x00": raise SystemExit("bad const type %r" % t)
    n = copyint()                                  # protos
    for _ in range(n): func()
    n = copyint(); out.extend(rd(4 * n))          # lineinfo
    n = copyint()                                  # locvars
    for _ in range(n): string(); copyint(); copyint()
    n = copyint()                                  # upvalue names
    for _ in range(n): string()
func()
assert p == len(src), (p, len(src))
open(sys.argv[2], "wb").write(out)
