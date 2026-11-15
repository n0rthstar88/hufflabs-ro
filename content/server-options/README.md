# Server options

Stock rAthena features and NPCs that are off by default, turned on for HuffLabs RO. `./deploy.sh`
installs everything on the game server container; `./deploy.sh --restart` also restarts the map
server (refuses while anyone is online). Safe to run again.

| What | How |
|---|---|
| `@autoloot`, `@alootid` for players | added to group 0 in `conf/import/groups.yml` |
| Monsters drop zeny | `zeny_from_mobs: yes` (block in `conf/import/battle_conf.txt`) |
| Day/night cycle | 2 h day, 30 min night (`day_duration`, `night_duration`) |
| Attendance calendar (client button) | `server/attendance.yml` from `gen-attendance.py`: one period per calendar month, day N of the month's check-ins gives N HuffLabs Coins (days 1-20). Periods run to the end of 2035; re-run the generator with later years before then. |
| Login streak | `server/npc/daily_streak.txt`: logging in on consecutive days grows a per-account streak (a missed day resets it). Each day gives roulette Bronze equal to the streak day (max 10), plus 5 Silver every 7th day. 30 / 90 / 180 / 365 days in a row pay 10 / 30 / 60 / 1000 each of Gold Coins, HuffLabs Coins and roulette Gold, and repeat every year of an unbroken streak. `@streak` shows it. |
| Roulette prizes | rAthena's official table (`sql-files/roulette_default_data.sql`), loaded once into `db_roulette` |
| Stock extras | Private MVP Room, MVP Arena (warper on the fountain steps), Monster Arena (moved from 158,193 to 166,192 so it doesn't crowd the fountain), Morroc Raceway, Reset Girl, Kafra bank clerks, lottery, blackjack, rock-paper-scissors roulette, marriage, Card Seller |
| Card Seller's tables | it reads `item_db_re` / `mob_db_re` from SQL; the deploy builds rAthena's `yaml2sql`, loads the official tables and restores the stock `sql-files/` |
| Battlegrounds-style PvP room | `npc/custom/battleground/bg_pvp.txt`, listed in `npc/scripts_custom.conf` (it must load after the stock map flags, or `gvg.txt` strips its battleground flag from `guild_vs3`) |
| Seasonal events by date | `server/events/`: `events.tsv` (installed as `/etc/hufflabs-events.tsv`) lists the newest official version of each holiday with its dates; `hlro-events.sh` (hourly timer) writes `npc/custom/hufflabs/events.conf`, imported by `npc/scripts_custom.conf`, and restarts the map server when the list changes and nobody is online. `hlro-events.sh --list` shows today's events. |

Not possible: the auction house (`feature.auction`); the client removed it in 2014.

Notes:
- `delnpc:` and anything that must override stock NPC files only works from `npc/scripts_custom.conf`:
  `conf/import/map_conf.txt` is read before the NPC lists, and its `npc:` lines load first.
- `StPatrick_2008.txt` logs ~420 "Invalid delay 0" warnings while it's loaded (10-24 March); rAthena uses 5 s, harmless.
- Moving Monster Arena edits a stock file; re-run the deploy after an rAthena update.
