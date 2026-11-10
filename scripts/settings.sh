# Sourced by the repo's scripts. Loads your settings (server address, Proxmox
# host, container id, …) from $HLRO_ENV, default ~/.config/hufflabs-ro/launcher.env,
# so no address lives in the code. See docs/setup.md for the variables.
HLRO_ENV=${HLRO_ENV:-$HOME/.config/hufflabs-ro/launcher.env}
# Values already set in the environment win over the file.
if [ -f "$HLRO_ENV" ]; then
	while IFS= read -r _line || [ -n "$_line" ]; do
		case $_line in '' | '#'*) continue ;; esac
		_k=${_line%%=*}
		[ -n "${!_k+x}" ] || export "$_k=${_line#*=}"
	done < "$HLRO_ENV"
	unset _line _k
fi
# need VAR... : stop with a clear message if a setting is missing
need() {
	local v
	for v in "$@"; do
		if [ -z "${!v:-}" ]; then
			echo "missing setting $v (set it in $HLRO_ENV or the environment; see docs/setup.md)" >&2
			exit 1
		fi
	done
}
