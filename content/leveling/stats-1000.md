# Stats at 1,000: formula check (Task 6)

2026-11-12, source review of rAthena `d4b8e7b` as built on the server (Renewal, RENEWAL_ASPD).
Nothing overflows, wraps or goes negative at level 500 with all six stats at 1,000, so no fix was
added to `rathena.patch`.

| Area | What the code does at 1,000 | Result |
|---|---|---|
| Field sizes | `status_data` stats, hit, flee, cri, flee2, def2, mdef2 are int16 (max 32,767); matk_min/max, watk uint16; batk int32. Worst case here ≈ 3,300 (cri = LUK×10/3). | safe |
| ASPD | `status_base_amotion_pc`: sqrt(DEX²/5 + AGI²/2)/4 + 196 ≈ 405 before caps; the amotion is then clamped to `max_third_aspd` 193 (`max_aspd` 190 for non-third) before it is stored. | capped at 193 |
| Variable cast | `1 - sqrt((DEX×2 + INT)/530)` goes negative above 530; `max((int)time, 0)` clamps it. Fixed cast is untouched. Same as official once DEX×2+INT ≥ 530. | 0, never negative |
| Max HP / SP | double math, clamped to uint32, then capped by `max_hp` 1,100,000 and `max_sp` 1,000,000. All 25 final classes use the HpFactor/HpIncrease formula (no explicit BaseHp table), so HP rises smoothly to 500 with no drop at 276. | capped, no wrap |
| ATK / MATK | Renewal status ATK = STR + DEX/5 + LUK/3 + Lv/4 + 5×POW ≈ 1,660; status MATK ≈ 2,160 (uint16 holds 65,535). Damage math is int64. | safe |
| Stat cost | Flat 40 from 130 on (patch); fits the client's uint8 "points needed" field. | safe |

Balance notes (not bugs):
- LUK near 1,000 gives about 100% perfect dodge (flee2 = LUK + 10, in tenths) and a crit rate far above 100%.
- AGI/DEX: ASPD hits the 193 cap long before 1,000.
- VIT/INT: max HP/SP hit the caps above, so very high VIT stops adding HP.
- Status-effect resistances based on VIT/INT/LUK reach full immunity.

In game 2026-11-12 (GM test character, Dragon Knight 500/250, all stats 1,000 + gear): ASPD 193, HP 1,100,000/1,100,000,
ATK 2,396 + 5,100, MATK 2,416 + 5,060, Hit 2,271, Flee 1,949 + 111, Def 1,026 + 7,116, Critical 352; Poring
hits fine. All as predicted.
- Cosmetic: the status window's 4-digit stats overlap the "+bonus" column.
- Cosmetic: "Status Point" shows at most 32,767: the client's status packet field is int16
  (`clif.cpp:4128` caps it); the server keeps the real total (268,306 on the test character). A normal player has 91,174
  points in all by 500, so it only shows when more than 32,767 sit unspent. Not patchable server-side.
