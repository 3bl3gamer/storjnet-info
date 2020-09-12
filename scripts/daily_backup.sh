#!/bin/bash
scripts_dirname=`dirname "$0"`
dumps_dirname=${1:-$scripts_dirname/../dumps}
mkdir -p "$dumps_dirname"
stamp=`date -Idate`
"$scripts_dirname/db_dump.sh" "$dumps_dirname/dump_$stamp.sql.gz"
find "$dumps_dirname" -type f ! -regex '.*/dump.*-\(01\|15\).sql.gz' ! -name "dump_$stamp.sql.gz" -exec rm {} \;
