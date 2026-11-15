# Server options

Stock rAthena features and NPCs that are off by default, turned on for HuffLabs RO. `./deploy.sh`
installs everything on the game server container; `./deploy.sh --restart` also restarts the map
server (refuses while anyone is online). Safe to run again.

| What | How |
|---|---|
| `@autoloot`, `@alootid` (`autolootitem`) for players | added to group 0 in `conf/import/groups.yml` |
| Monsters drop zeny | `zeny_from_mobs: yes` (block in `conf/import/battle_conf.txt`) |
| Day/night cycle | 2 h day, 30 min night (`day_duration`, `night_duration`) |
| Attendance calendar (client button) | `server/attendance.yml` from `gen-attendance.py`: one period per calendar month, day N of the month's check-ins gives N HuffLabs Coins (days 1-20). Periods run to the end of 2035; re-run the generator with later years before then. |
| Login streak | `server/npc/daily_streak.txt`: logging in on consecutive days grows a per-account streak (a missed day resets it). Each day gives roulette Bronze equal to the streak day (max 10), plus 5 Silver every 7th day. 30 / 90 / 180 / 365 days in a row pay 10 / 30 / 60 / 1000 each of Gold Coins, HuffLabs Coins and roulette Gold, and repeat every year of an unbroken streak. `@streak` shows it. |
| Roulette prizes | rAthena's official table (`sql-files/roulette_default_data.sql`), loaded once into `db_roulette` |
| Stock extras | Private MVP Room, MVP Arena (warper on the fountain steps), Monster Arena (moved from 158,193 to 166,192 so it doesn't crowd the fountain), Morroc Raceway, Reset Girl, Kafra bank clerks, lottery, blackjack, rock-paper-scissors roulette, marriage, Card Seller |
| Card Seller's tables | it reads `item_db_re` / `mob_db_re` from SQL; the deploy builds rAthena's `yaml2sql`, loads the official tables and restores the stock `sql-files/` |
| Battlegrounds-style PvP room | `npc/custom/battleground/bg_pvp.txt`, listed in `npc/scripts_custom.conf` (it must load after the stock map flags, or `gvg.txt` strips its battleground flag from `guild_vs3`) |
| Seasonal events by date | `server/events/`: `events.tsv` (installed as `/etc/hufflabs-events.tsv`) lists the newest official version of each holiday with its dates; `hlro-events.sh` (hourly timer) writes `npc/custom/hufflabs/events.conf`, imported by `npc/scripts_custom.conf`, and restarts the map server when the list changes and nobody is online. `hlro-events.sh --list` shows today's events. |
| Seasonal town weather | `server/npc/town_weather.txt`: 30 towns each roll weather for 1.5-3 h, weighted by the real season (winter snow, spring sakura, fall leaves, fog, clouds, mist; desert/tropical towns stay warm; no rain: the 2025 client has no rain weather). Lutie always snows, Niflheim is always foggy, Prontera gets fireworks on holiday evenings. Needs `rathena-weather.patch` (new `refreshweather` script command: new weather shows at once, ending weather reloads the map for players in that town). The deploy applies it once and rebuilds the map server. GM: `@townweather`, `@townweather <town> <kind>`, `@townweather reroll`. |
| GM panel | `server/npc/gm_panel.txt`: `@gm`, or "GM panel" at the top of `@menu` (GM level 99 only). Players (go to, bring, heal, revive, give items / HuffLabs Coins / zeny, info incl. login streak, mute, jail, kick), broadcasts, world (town weather, day/night, WoE start/end/controller, how events and seasons work), monsters and items (spawn, kill all, search, info, who drops, get item, clean map, revive map), your own character (heal, hide, speed, storage, go/warp, level up, autoloot, refresh), server (who, users, map info, reload scripts, kick all) and a GM command list. Everything runs through normal @/# commands, so permissions and logs apply. The deploy also installs content/leveling's `menu.txt`, which lists `@autoloot`, `@alootid` and `@streak` for players. |

Not possible: the auction house (`feature.auction`); the client removed it in 2014.

Notes:
- `delnpc:` and anything that must override stock NPC files only works from `npc/scripts_custom.conf`:
  `conf/import/map_conf.txt` is read before the NPC lists, and its `npc:` lines load first.
- `StPatrick_2008.txt` logs ~420 "Invalid delay 0" warnings while it's loaded (10-24 March); rAthena uses 5 s, harmless.
- Moving Monster Arena edits a stock file; re-run the deploy after an rAthena update.
