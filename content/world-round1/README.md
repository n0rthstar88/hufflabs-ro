# World round 1

Community NPCs and small source changes from rathena.org, picked by the server owner and
adapted for HuffLabs RO. The downloads themselves stay in the quarantine folder
(`~/Games/Ragnarok/custom-downloads/picks/`) and are not committed; `build-npcs.py` and
`build-client.py` rebuild everything from there.

## What it adds

| Feature | Where | Notes |
|---|---|---|
| Punching Bags (Secretdataz) | Izlude 125–128, 125–131 | Five dummies that call out your DPS every 5 s. |
| Build Manager (Emistry) | Juno 162,56, `@bm` | Save up to 3 stat builds. **Fixed:** loading no longer resets skills (upstream wiped them). |
| Advanced Stylist 4.4 | Prontera 160,182 | One NPC (upstream had three duplicates stacked on one cell). |
| Sader's Enchantment 3.1 | Geffen 124,61 | Any gear (upstream sample only allowed novice items), never breaks gear (upstream 50%). |
| Dress Room (Secret, MIT) | Morroc 167,95 | Unchanged. |
| Item Mall | Warper in Amatsu 200,246 → `quiz_02` | Unchanged, everything for zeny (owner's choice). All 3,696 item ids exist in our item db. |
| WoE controller (Euphy 1.9 + BeWan 1.4) | Prontera 144,198, `@woe`, `@woetime` | No schedule until a GM adds sessions in its menu; stock agit schedulers are `delnpc`'d. Castle reward: 10 HuffLabs Coins. |
| Mini Endless Tower (Sader1992) | Alberta 191,145 | 25 floors, once a day. Map `1@mtower` = copy of `1@tower` (server map cache import + client resnametable alias). Reward per clear: 20 coins + 10% of next base/job level. |
| Hunting Quest 5.2 (Sader1992) | Payon 165,225, `@myquest` | Five tiers by level (1–500) with monsters that spawn at those levels. Points buy HuffLabs Coins, 10 points = 1 coin. |
| Costume Enchant (Dimas_Shotta93) | Al De Baran 142,123 | 10 HuffLabs Coins per enchant (upstream: Silvervine Fruit, which nothing here sells). |
| Mob Element View (Hyroshima) | source | Monsters show their element as a title and icon. **Fixed:** Poison used the Ghost icon. |
| MVP tombs (DynamicMVPTomb) | source + client | Tombs show the MVP's name and, for 91 MVPs, a tomb sprite of that MVP (NPC ids 19100–19190). |
| MVP / mini-boss card announce | source | Server-wide message when one drops. Rewritten from Bad's patch: no SQL logging (it built SQL from unescaped names), no fixed 128-byte buffer, message 1036. Colours: `mvp_card_announce_color`, `miniboss_card_announce_color` (0 = off). |
| Charm items | source | Item type `Charm`: its script applies while it sits in the inventory. No charm items yet. |
| Refine announce | `db/import/refine.yml` | Server-wide message for refines from +10 (official: +13). Uses rAthena's built-in broadcast; the Athena Studio patch was broken (bad hunk sizes, missing `#endif`) and is not needed. |

`@menu` (content/leveling) lists the new commands and a "Town services" page that can take you to each NPC.

## Files

- `rathena.patch`: the source changes, applied by `deploy.sh` (git apply, once).
- `server/npc/*.txt`: built by `build-npcs.py <picks> server/npc`. Not in the public repo: they are
  edited copies of the authors' scripts, most of which carry no licence, so download the originals
  from rathena.org and build them yourself before running `deploy.sh`.
- `server/db/import/*.yml`, `server/build_manager.sql`, `server/mapcache-alias.py`.
- `build-client.py <picks> <client> <grftool> <scratch>`: element icons, tomb sprites, plain-Lua
  `npcidentity.lub` / `jobname.lub` (dumped from data.grf with `lub32to64.py` + `lubdump.lua`,
  then the tomb ids appended), and `data/resnametable.txt` with the `1@mtower` alias.

## Credits

Punching Bag: Secretdataz. Build Manager: Emistry. Advanced Stylist: rAthena community.
Sader's Enchantment, Mini Endless Tower, Hunting Quest: Sader1992. Dress Room: Secret (Jittapan P., MIT).
WoE controller: Euphy, Lemongrass, Capuche, BeWan. Costume enchant: Dimas_Shotta93.
Mob Element View: Hyroshima. DynamicMVPTomb: its rathena.org author. Card announce idea: Gabriel "Bad" dos Prazeres.
Charm items: eduardo.schmidte. Item Mall: rathena.org download `mall.txt`.
Art in the client files belongs to its makers; it ships to friends only, never to the public repo.
