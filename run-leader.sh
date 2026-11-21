#!/bin/sh
set -eu
HERE="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
SPAR_PROFILE=leader exec "$HERE/harrier"
