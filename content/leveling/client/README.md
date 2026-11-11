# Client: XP bars past 275

`ExpSpec.yml` is the client's `Inputs/ExpSpec.yml` with one extra entry at the end: the 25 final
classes (job IDs 4252–4264, 4278–4281, 4302–4308, 4316) get base level 500 and job level 250 bars.

`hufflabs.yml` is the WARP profile: the 2026-11-09 session (37 patches) plus `CustomBarLimits`
with `$expBarSpec = Inputs/ExpSpec.yml`. To re-patch, from the client folder:

    export STEAM_COMPAT_DATA_PATH=~/.local/share/proton-prefixes/ragnarok STEAM_COMPAT_CLIENT_INSTALL_PATH=~/.local/share/Steam
    "$HOME/.local/share/Steam/steamapps/common/Proton - Experimental/proton" run cmd /c 'win32\WARP_console.exe -using profiles\hufflabs.yml > warp_out.txt 2>&1'

Use Windows paths (`win32\…`, `profiles\…`): with forward slashes Proton can't find the exe and
WARP exits silently. Output: `Outputs/patched.exe` (2026-11-10: 38/38 patches), copied to
`HuffLabsRO.exe`. Never rcedit it. Backups live in `~/Games/Ragnarok/backups/`.
