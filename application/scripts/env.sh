# Source this file to make the project's built commands available on PATH.
# It works from any current directory.

_bdfgen_project_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
export BDFGEN_HOME="${_bdfgen_project_root}/build"
case ":${PATH}:" in
	*":${BDFGEN_HOME}:"*) ;;
	*) export PATH="${BDFGEN_HOME}:${PATH}" ;;
esac
unset _bdfgen_project_root
