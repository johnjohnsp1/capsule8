#!/bin/bash
#
# CI build script
#

set -xeo pipefail

[[ -n $DEBUG ]] && set -o xtrace

function on_error() {
	if [[ $? -ne 0 ]]; then
		echo "^^^ +++"
	fi
}

trap on_error EXIT

#
# Check if sources are buildable and vet-clean first since this runs
# faster than compiling static binaries
#
echo "--- Checking sources"
make check

echo "--- Building container"
make container
