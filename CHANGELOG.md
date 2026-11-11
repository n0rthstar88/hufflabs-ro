# Changelog

Each version is tagged and has a [release](https://github.com/n0rthstar88/hufflabs-ro/releases)
with built tools. Every version passes the [tests](https://github.com/n0rthstar88/hufflabs-ro/actions/workflows/ci.yml)
on fresh Linux and Windows machines first.

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
