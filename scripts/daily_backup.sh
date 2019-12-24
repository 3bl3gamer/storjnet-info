#!/bin/bash
scripts_dirname=`dirname "$0"`
dumps_dirname=${1:-$scripts_dirname/../dumps}
mkdir -p "$dumps_dirname"
"$scripts_dirname/db_dump.sh" "$dumps_dirname/dump_`date -Idate`.sql.gz"