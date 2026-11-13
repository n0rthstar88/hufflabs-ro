# World round 4: costume headgears and dyes

- **354 costume headgears** with no stats, sold for 200 HuffLabs Coins by three Costume Tailors in
  Alberta (upper, middle, lower). `make-headgears.py <downloads>` builds `headgears.tsv` (item id,
  look id, sprite, slots, name, source) from Mihael's custom item pack plus the Valentine Ring, Flames
  and Light Star Aura recolours; `names_en.tsv` gives English names. Items 2601000+, looks 2936+.
- `build-server.py` writes the item db import and the Tailor shops; `build-client.py` copies the
  sprites (as `_hlro_<name>`) and icons and adds the look ids. Item names come from
  content/coin-shop's `build-client.py`, which reads `headgears.tsv`.
- **Dyes:** 553 clothes and 251 hair colours from the `553-ClothesDyes-251HairDyes.grf` pack, loaded
  after data.grf so official palettes win. `server/npc/dyes.txt` keeps the stylist within range and
  `deploy.sh` raises `max_hair_color` / `max_cloth_color`.

## Credits

Custom headgears: Mihael. Light Star Aura: Kinx, shared here with his permission. Valentine Ring,
Flames and the dye pack: their rathena.org authors. Art ships to friends only, never to this repository.
