#!/bin/bash
fname=${1:-dump.sql.gz}
echo "dumping DB to $fname"

pg_dump --username=storjinfo --schema=storjinfo --format=p storjinfo_db --clean --if-exists | gzip -5 --force > "$fname"

echo -n "done, "
du -h "$fname" | cut -f1
