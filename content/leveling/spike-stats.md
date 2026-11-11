# Spike: stats above 255 on the 2025-07-16 client (Task 1)

2026-11-10, tested in game (GM, Dragon Knight 500/250): `@stpoint 100000`, `@str 300`, `@agi 250`, then `+`
on AGI past 255 — server shows STR 638, AGI 601. The status window shows the real values (above 255),
the `+` button works past 255, and the values still show correctly after logging out and in.

Result: **keep the stat cap at 1,000**.
Also confirmed: base 500 / job 250 XP bars draw normally with the CustomBarLimits patch.
