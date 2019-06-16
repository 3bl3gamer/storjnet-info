#!/bin/bash
fname=${1:-dump.sql.gz}
echo "restoring DB from $fname"

zcat "$fname" | psql --set=ON_ERROR_STOP=1 --single-transaction --username=storjinfo storjinfo_db

echo "done"
