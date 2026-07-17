#!/bin/sh

set -eu

tracked_local_data=$(
	git ls-files --cached | awk '
		/^tmp\// || /\.(db|sqlite|sqlite3)(-(shm|wal))?$/ { print }
	'
)

if [ -n "$tracked_local_data" ]; then
	printf '%s\n' "Error: local database files must not be tracked by Git:" >&2
	printf '%s\n' "$tracked_local_data" >&2
	printf '%s\n' "Remove them from the Git index before committing." >&2
	exit 1
fi

printf '%s\n' "Repository check passed: no local database files are tracked."
