# Leveling project (server side)

| File | What it is |
|---|---|
| `rathena.patch` | Source patch against rAthena `d4b8e7b`: `MAX_LEVEL 500`, 15x XP when the character variable `HLRO_PACE` is 15, no level penalty above base level 275, and a flat cost of 40 per stat point from 130 up. Re-apply after any rAthena upgrade. |
| `gen_tables.py` | Writes `server/db/import/job_stats.yml` (XP tables for the 25 final classes: base 500, job 250) and `server/db/import/statpoint.yml` from copies of the server's official tables in `upstream/` (not committed; copy `db/re/{job_exp,statpoint,mob_db}.yml` there). Tests: `python3 -I -m unittest discover -s tests`. |
| `server/battle_conf.txt` | Stat caps (1000), installed as a marked block in `conf/import/battle_conf.txt`. |
| `server/db/import/*.yml` | Generated import tables (committed so the server can be rebuilt from the repo). |
| `server/npc/*.txt` | NPC scripts installed in `npc/custom/hufflabs/`. |
| `deploy.sh` | Installs all of it in the game server container (`HLRO_CT`). `--build` rebuilds rAthena (about 30 min), `--restart` restarts login, char and map (refuses while anyone is online). Every overwritten file gets a `.bak-<time>` copy next to it. |

Roll back: on the game server, `git -c safe.directory='*' apply -R` the patch, rebuild, and restore the `.bak-` copies of the touched files.
