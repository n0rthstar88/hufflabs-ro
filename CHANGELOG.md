# Changelog

Each version is tagged and has a [release](https://github.com/n0rthstar88/hufflabs-ro/releases)
with built tools. Every version passes the [tests](https://github.com/n0rthstar88/hufflabs-ro/actions/workflows/ci.yml)
on fresh Linux and Windows machines first.

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
