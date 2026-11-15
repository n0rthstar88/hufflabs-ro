# Changelog

Each version is tagged and has a [release](https://github.com/n0rthstar88/hufflabs-ro/releases)
with built tools. Every version passes the [tests](https://github.com/n0rthstar88/hufflabs-ro/actions/workflows/ci.yml)
on fresh Linux and Windows machines first.

## v1.4.1

**Prontera's season switches on the server; the publisher's PC can be off.**

- `hufflabs-ro-publish --seasons <dir>` (or `HLRO_SEASONS`) signs one client manifest per
  season with the same serial and uploads them to `update/seasons/`. The live manifest is
  always a copy of the server's current season, made under a lock shared with the switch.
- New `content/world-round3/server/season/` + `install-season-switch.sh`: an hourly timer
  on the game server switches the client manifest and Prontera's map cache on the season's
  date, once nobody is online. The signing key stays on the publisher's PC.
- `build-season-sets.py` builds the four season file sets; `season.sh` now only keeps the
  publisher's own client in step (`auto`), forces a switch (`<season>`) or shows `status`.
- A plain publish is refused once the server switches seasons, so stale season manifests
  can't roll launchers back.

## v1.4.0

**Server options: everything stock rAthena leaves off, switched on.**

New `content/server-options/` folder with a deploy script and README:

- **Players get `@autoloot` and `@alootid`.** Monsters drop zeny, and there is a day/night
  cycle (2 h day, 30 min night).
- **Attendance that grows.** The client's attendance calendar gives one more HuffLabs Coin for
  every check-in in a month (day 1 = 1 coin ... day 20 = 20 coins); `gen-attendance.py` writes
  the monthly periods.
- **Login streaks.** Logging in on consecutive days builds a per-account streak (`@streak`):
  roulette Bronze that grows each day, Silver every 7th day, and perfect-attendance rewards at
  30 / 90 / 180 / 365 days (10 / 30 / 60 / 1000 each of Gold Coins, HuffLabs Coins and roulette
  Gold). A missed day starts it again.
- **Roulette** gets rAthena's official prize table.
- **Stock extras:** Private MVP Room, MVP Arena, Monster Arena (moved off the crowded fountain),
  Morroc Raceway, Reset Girl, Kafra bank, lottery, blackjack, rock-paper-scissors, marriage and
  the Card Seller, whose SQL item tables the deploy builds with rAthena's `yaml2sql`.
- **Opt-in PvP:** a Red vs Blue team arena next to the stock PvP rooms. It loads from
  `npc/scripts_custom.conf` so the stock map flags can't override it.
- **Seasonal events by date:** the newest official version of ten holidays (Lunar New Year to
  Christmas) loads automatically on its dates; an hourly timer restarts the map server when the
  list changes and nobody is online.

## v1.3.2

**Fixes: WoE no longer runs on its own, NPC looks stand on the ground, a job changer at the fountain.**

- **WoE schedule really off.** The stock War of Emperium schedulers were never unloaded:
  `delnpc` lines in `conf/import/map_conf.txt` do nothing, because that file is read before
  any NPC list exists, and `npc/scripts_guild.conf` lists `agit_controller.txt` with a
  trailing space. WoE kept starting Tue/Thu 21:00 and Sat 16:00. The round-1 deploy script
  now puts the `delnpc` lines in `npc/scripts_custom.conf` and removes the old ones.
- **NPC looks fixed from every angle.** The round-3 NPC looks only had a correct
  south-facing picture; the other seven directions used template offsets, so NPCs sank
  into the floor or slid sideways when turned or seen from another camera angle.
  `fix-npc-act.py` (run by `build-client.py`) gives every direction the good picture.
- **Prontera Seasons ground fix.** `seasons-names.py` could map two floor textures onto the
  same name and leave others missing (black ground in Prontera). It now decodes names by a
  per-character vote and refuses collisions. `build-client.py` also keeps the client's
  existing file-name case, so the update manifest never lists a file twice.
- **Town NPCs.** The stock rAthena Job Master is on, front and centre at the Prontera
  fountain (156,193), and the healers in every town use the nurse sprite instead of a Poring.

## v1.3.1

**A bigger world: community NPCs, coin vending, seasons, 345 costumes.**

Five content rounds built from free rathena.org community releases, adapted and fixed for
HuffLabs RO. Each `content/world-round*/` folder has a README, a deploy script and the
builders that recreate everything from the downloads. The downloads themselves (art and the
authors' scripts) are not in this repository.

- **Round 1:** ten community NPCs spread over the towns (punching bags, Build Manager,
  stylist, enchanter, dress room, item mall, WoE controller, Mini Endless Tower, hunting
  quest, costume enchant) and hand-placed source changes: monster element view, MVP tombs
  with the MVP's sprite, MVP card announcements and charm items. Refines from +10 are
  announced. The Super Saiyan Aura costume joins the coin shop.
- **Round 2:** vendors pick Zeny or HuffLabs Coins as their shop's currency (a rewrite of
  "Extended Vending" without its item-id, duplication and buffer bugs), and random option
  drops on 792 monsters.
- **Round 3:** Prontera follows the real season (`season.sh`, run daily by a timer), 25
  Poring Family monsters roam the fields and 10 townsfolk use new NPC looks.
- **Round 4:** 345 stat-less costume headgears at three Costume Tailors in Alberta, and
  553 clothes / 251 hair dyes.
- **Round 5:** a fill-only installer for missing item art, HD damage numbers and two new
  cart looks.
- `@menu` lists the new commands and can take you to each town service.

## v1.2.2

**Account settings and a command menu.**

- **The launcher relays the game's web service.** The client loads account settings
  (hotkeys, window layout) and guild emblems from `127.0.0.1:8888` on the player's own
  PC, so with only the game ports open it showed "There was an error when loading the
  data account settings". While the game runs, the launcher now forwards that port to
  rAthena's `web-server` next to the game server (the address comes from the client's own
  `data/sclientinfo.xml`) and stops when the game closes. A busy port or a missing
  address is logged and the game starts anyway. Friends need `tcp:8888` to the game
  server in the Tailscale policy (`tailscale/policy-snippet.hujson`).
- **`@menu`** (`content/leveling/server/npc/menu.txt`): every command a normal player can
  use, explained; server commands can be run from the menu (`@go` asks for a town, `@warp`
  for a map). The client's Cash Shop button and a new HUD button (WARP `CustomButtons`)
  open it, through a small hook in `rathena.patch`, since the server has no cash shop.
- **Healers in every town**: rAthena's stock healer plus one for Lasagna.
- **Setup guide page** at `/setup` on the sign-up site, linked from the sign-up page and
  from the invite message.
- **Leveling:** a formula check at 1,000 stats (`content/leveling/stats-1000.md`):
  nothing overflows; attack speed, cast time and max HP/SP stop at their caps. The
  deploy script gives every patched source file back to the `rathena` user.

Launcher version 1.0.4.

## v1.2.1

**Windows fixes**, found on the first real Windows installs.

- **Setup unpacks with .NET's ZipFile instead of `tar.exe`.** Windows' `tar.exe` reads
  file names in the local code page, so on non-Korean Windows the client's Korean file
  names came back unreadable and setup stopped.
- **The launcher can start the game on Windows.** The client's manifest asks for admin
  rights, and the launcher used to start it in a way that can't show the Windows admin
  prompt, so every start failed with "requires elevation". It now goes through
  `ShellExecuteEx`, which shows the prompt (Wine/Proton never enforced this, so it only
  showed up on Windows).
- **Start failures say why**: game file missing, blocked by antivirus, blocked by Smart
  App Control or a policy, the admin prompt answered No, or Windows' own message. Retry
  now checks the files again first, so a game file that antivirus removed is downloaded
  again. Longer messages wrap onto up to three lines.
- **Docs**: the client's `libcurl.dll` needs `MSVCR110.dll` (Visual C++ 2012), which a
  fresh Windows doesn't have and Wine supplies by itself; see the setup guide's gotchas.

Launcher version 1.0.3.

## v1.2.0

**Leveling** (`content/leveling/`): a longer game for players who want one, without
forcing it on anyone.

- **Pick your pace**: at first login each character chooses normal or 15x XP in a popup.
  The choice is locked once made; `@pace` shows it.
- **Max level 500 / job 250** for the final class of each line, with no level penalty
  above base level 275. New XP curves follow the official ones through level 209 (job
  29), then rise so 275 → 500 takes about 40 hours at 15x.
- **Stats up to 1,000**, with a flat cost of 40 points per stat from 130 up. Checked in
  game: the 2025-07-16 client shows values above 255 correctly.
- **Tougher MVPs** by tier (more HP and XP), and MVP coin drops of 30 / 60 / 100 by tier.
- **Client XP bars** to 500/250 through the WARP `CustomBarLimits` patch (profile and
  `ExpSpec.yml` included).
- Ships as a small rAthena source patch (against rAthena `d4b8e7b`), generated import
  tables, NPC scripts and a deploy script. Tests check the generated tables against
  rAthena's official ones, downloaded fresh on each run.

The server side is live. Some balance checks are still open (exact 15x XP rates, mixed
parties, rebirth), so numbers may change in a later version.

## v1.1.0

**HuffLabs Coin and the coin shop** (`content/coin-shop/`).

- A new item, HuffLabs Coin, drops from monsters: a 0.2% chance at level 1 rising to 3%
  at level 150 and up. Bosses always drop several, MVPs the most, shared with the party
  on the same map. Monsters that give no base EXP never drop coins.
- Two merchants in Prontera sell for coins: a **Costume Merchant** (45 official costume
  headgears that weren't obtainable on the server) and a **Gear Merchant** (new stat
  gear on official art).
- Coins can be traded, so friends can pool them.
- Ships as rAthena import files (`item_db.yml`, NPC scripts), the client item
  descriptions, and a deploy script that is safe to run again.

## v1.0.0

The first public version: everything a small group of friends needs to sign up, install
and stay up to date.

- **Sign-up site** (Go): invite codes, account creation in rAthena's `login` table, an
  admin page, the download page and the launcher's update files. Runs behind
  `tailscale serve`.
- **Launcher**: a small self-drawn Windows window that checks an ed25519-signed file
  list, downloads changed files with resume, swaps them in atomically, updates itself,
  then starts the game. Linux runs the same exe through Proton.
- **Publish tool**: builds the file list from your client folder plus the theme,
  uploads only new files, then switches the live list.
- **Installers** for Windows (NSIS) and Linux: download the client once with resume and
  a checksum check, write default settings, install the launcher and shortcuts.
- **Client theme builder**: login screen, rules page and dialogs from your own artwork.
- **Tests on GitHub**: Go tests (including the database tests against a throwaway
  MariaDB with rAthena's real `login` table), the Linux installer, the release
  switch-over, and the Windows installer's folder and registry logic on real Windows.

Downloads: the portal server, the publish tool, `themetool` and `grftool`, and the Linux
installer template. The launcher and the Windows installer aren't attached: they're
built with your own site address and signing keys (see `docs/setup.md`).
