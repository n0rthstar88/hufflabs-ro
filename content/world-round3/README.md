# World round 3: seasons, the Poring Family and new NPC looks

- **Prontera follows the real season.** `season.sh <spring|summer|fall|winter|auto>` swaps in that
  season's Prontera map and models from Olrox's Prontera Seasons, on the client (then publish an update)
  and in the server map cache (`server/mapcache-gat.py`). `auto` picks the season from today's date and
  exits 75 (try again later) while the game runs locally or anyone is online, so a daily systemd timer
  can call it early in the morning. `seasons-names.py` recovers the zip's mangled Korean file names.
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
