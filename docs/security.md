# Security notes

HuffLabs RO is for a handful of friends on a private tailnet. This is what protects
them and the server, and what doesn't.

## The update channel

- The launcher only accepts a file list (`/update/manifest.json`) signed with one of
  the ed25519 keys built into it, and refuses a list older than the one it has
  installed. A compromised site can't push new files through the launcher without a
  private key; the keys never touch the server.
- Every downloaded file is checked against its SHA-256 before it replaces anything,
  and replaced atomically. Paths in the list must be relative, can't climb out of the
  install folder, can't use names Windows reserves, and can't touch the player's own
  folders.
- The publish tool checks on the server that the list and the upload arrived whole
  before switching anything, and refuses to publish a launcher built for another
  site or key.

**Not covered by the signature:** the first install. `HuffLabsRO-Setup.exe`, the Linux
installer, the client archive and the launcher the Linux installer downloads all come
from the site and are checked only against checksums served by the same site.
Someone who controls the site can change those for new installs (and for anyone who
re-runs the Linux installer). The site is reachable only on the tailnet.

## The site

- Reachable only through `tailscale serve`; friends can reach only the site and the
  game ports (see the policy grant).
- Sign-up needs a single-use invite code (stored hashed). Repeated wrong codes and
  wrong admin passwords lock an address out for 10 minutes.
- Admin sessions are HMAC-signed, `HttpOnly`, `Secure`, `SameSite=Strict` cookies with
  a CSRF token on every admin form. SQL is parameterised.
- The site's database user can only read and add rows in `ragnarok.login`.
- rAthena stores passwords as MD5 because that's what its login server supports; the
  game protocol limits passwords to 23 characters.

## Windows installer

- Runs elevated, so system tools are called by full path, the uninstaller is made
  read-only for normal users, and removing folders never follows junctions or
  symlinks.
- Gives the built-in Users group write access to the game folder so the launcher can
  update without a UAC prompt. On a shared PC, any local user can change the game's
  files.
- Neither the installer nor the launcher is code-signed.

## Reporting

Open an issue on GitHub, or for something sensitive, use the contact on the
repository owner's profile.
