#!/bin/bash
fname=${1:-dump.sql.gz}
echo "dumping DB to $fname"

pg_dump --username=storjnet --schema=storjnet --format=p storjnet_db --clean --if-exists | grep -vP "(DROP SCHEMA|CREATE SCHEMA)" | gzip -5 --force > "$fname"

echo -n "done, "
du -h "$fname" | cut -f1
