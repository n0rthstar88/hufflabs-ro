# World round 3: seasons, the Poring Family and new NPC looks

- **Prontera follows the real season, switched by the server.** The publisher's PC doesn't need to
  be on. `build-season-sets.py` builds the four Prontera file sets from Olrox's Prontera Seasons
  (`seasons-names.py` recovers the zip's mangled Korean file names), and `hufflabs-ro-publish --seasons
  <dir>` (or `HLRO_SEASONS`) signs one client manifest per season, all with the same serial, into
  `update/seasons/` on the server. `install-season-switch.sh` installs `server/season/`: an hourly timer
  that, on the season's date (winter Dec 21, spring Mar 20, summer Jun 21, fall Sep 22) and once nobody
  is online, points the live manifest at that season, updates Prontera in the server map cache
  (`server/mapcache-gat.py`) and restarts the map server. The signing key never leaves the publisher's
  PC. Once the server switches seasons, a publish without the season sets is refused (it would leave
  stale season manifests that roll launchers back). `season.sh auto` keeps the publisher's own client
  folder on the server's season; `season.sh <season>` forces a switch; `season.sh status` shows it.
- **Poring Family:** 25 roaming monsters (`porings.tsv`, ids 31000–31024), with field spawns and a boss.
  `build-server.py` writes their mob db (loaded through content/leveling's `db/import/mob_db.yml` Footer)
  and spawns.
- **New NPC looks:** 94 looks from the 100 NPCs pack (`npc_looks.tsv`, ids 19200+), and 10 townsfolk
  that use them. The sprites have a single standing pose, so they work as NPCs, not playable classes.
- `build-client.py <picks> <client> <grf list> <season>` copies the art from the downloads folder and
  adds the ids to the plain-Lua `npcidentity.lub` / `jobname.lub` written by world-round1.

## Credits

Prontera Seasons: Olrox (free, non-commercial, with credit). Poring Family and 100 NPCs: their
rathena.org authors. Art ships to friends only, never to this repository.
