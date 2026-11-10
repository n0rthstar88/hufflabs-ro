# Setting up your own HuffLabs RO

This is how the pieces were set up for one small server. It assumes you're
comfortable with Linux, Proxmox and Tailscale; adapt freely.

## What you need

- **A game server:** [rAthena](https://github.com/rathena/rathena) built and running
  (`rathena-login`, `rathena-char`, `rathena-map` systemd units, game database
  `ragnarok`) with MariaDB, in a Proxmox container. The scripts reach it with
  `ssh <proxmox host> pct exec <container id> -- …`, and install rAthena's checkout
  at `/home/rathena/rathena`.
- **Tailscale** on the container, with HTTPS certificates on, tagged `tag:Game`, and
  the grant in [`tailscale/policy-snippet.hujson`](../tailscale/policy-snippet.hujson).
  An OAuth client with only the `device_invites` scope lets the site check share
  links.
- **Your own Ragnarok Online client** matching your rAthena PACKETVER (this project
  used the 2025-07-16 Ragexe, patched with WARP). Nothing from the client is in this
  repository.
- **On your PC:** Go 1.27, NSIS 3 (`makensis`), `bsdtar`, ImageMagick, and for the
  tests Wine and `umu-launcher`.

## Settings

Everything that names your setup lives in one file outside the repository,
`~/.config/hufflabs-ro/launcher.env` (or wherever `HLRO_ENV` points). Values in the
environment win over the file.

| Setting | Used for |
|---|---|
| `HLRO_SITE_URL` | The site, e.g. `https://game.your-tailnet.ts.net`. Built into the launcher and installers. |
| `HLRO_TRUSTED_KEYS` | Comma-separated base64 ed25519 public keys the launcher accepts updates from. |
| `HLRO_PROXMOX` | ssh target of the Proxmox host, e.g. `root@proxmox.lan`. |
| `HLRO_CT` | The container's id. |
| `HLRO_REPO` | This checkout (used by the publish wrapper). |
| `HLRO_BACKUP_JOB` | Optional: a Proxmox backup job to add the container to. |
| `HLRO_ADMIN_NAME` | Optional: the name on the in-game rules page ("Tell … instead of using it"). |
| `HLRO_UPDATE_DIR` | Optional: the update folder on the server (default `/srv/hufflabs-portal/update`). |

## 1. The site

```bash
deploy/server-setup.sh   # once: portal user and folders, MD5 passwords, portal database, portal.env
```

This switches rAthena to MD5 passwords (backing up the `login` table first), creates
the `hufflabs_portal` database and a `portal` MariaDB user that can only read and add
rows in `ragnarok.login`, and writes `/etc/hufflabs-portal/portal.env`. Fill in
`TS_CLIENT_ID` and `TS_CLIENT_SECRET` there, then:

```bash
deploy/deploy.sh         # build and install the site, start it once the OAuth client is set
```

Publish it on the tailnet with `tailscale serve --bg 8080` in the container. Admin
logins are rAthena accounts in GM group 99.

## 2. Signing keys

```bash
go run ./cmd/hufflabs-ro-publish keygen                                    # ~/.config/hufflabs-ro/publish.key
go run ./cmd/hufflabs-ro-publish keygen --out ~/.config/hufflabs-ro/publish-backup.key
```

Put both printed public keys in `HLRO_TRUSTED_KEYS`. Keep the backup private key
offline (a password manager); with it you can still publish if the main key is lost.
Without either, launchers can't be updated any more.

## 3. Theme and launcher

```bash
ART=~/Pictures CLIENT=~/Games/Ragnarok/Client theme/build-theme.sh   # needs your three art images, see the script
launcher/build.sh
```

## 4. First release

```bash
client/build-archive.sh 2026.11.09        # the first-install archive + Linux installer + launcher
installer/windows/fetch-redist.sh         # Microsoft runtimes bundled into Setup.exe (checksums pinned)
makensis -DSITE_URL="$HLRO_SITE_URL" -DVERSION=2026.11.09 installer/windows/hufflabs-ro.nsi
client/publish-installers.sh              # Setup.exe, Linux installer and launcher onto the download page
```

## 5. Publishing changes

Change your client folder or rebuild the theme, then:

```bash
hufflabs-ro-publish            # or: go run ./cmd/hufflabs-ro-publish
hufflabs-ro-publish --dry-run  # what would go up
hufflabs-ro-publish --list     # every file that ships
```

Files listed in `client/excludes.txt` never ship, and neither do the player's own
folders (`savedata/`, `memo/`, `Replay/`, `Name/`, `ScreenShot/`, `Chat/`,
`AI/USER_AI/`, logs). Publishing a new launcher build updates every friend's launcher
on its next start.

## Gotchas

- The client spins with no window under Wine/Proton until the `Gravity Soft\RenewSetup`
  registry key exists; the installers write it.
- Theme files go loose under `data/texture/À¯ÀúÀÎÅÍÆäÀÌ½º/` (the Korean folder name as
  the English-locale client asks for it). TGAs must be stored bottom-up.
- `rathena.target` doesn't stop or start the login and char servers; restart the three
  units individually.
