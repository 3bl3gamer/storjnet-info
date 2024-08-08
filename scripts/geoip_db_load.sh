#!/bin/bash
set -e

repo_dir=`dirname "$0"`"/.."
edition_id=$1

if [ -z "$edition_id" ]; then
    echo "<edition_id> argument is required (GeoLite2-City, GeoLite2-ASN, etc.)"
    exit 1
fi

if [ -z "$MAXMIND_KEY" ]; then
    echo "MAXMIND_KEY env variable is required"
    exit 1
fi

curl --silent --show-error --fail --location "https://download.maxmind.com/app/geoip_download?edition_id=$edition_id&license_key=$MAXMIND_KEY&suffix=tar.gz" \
    | tar --extract --gzip --to-stdout --wildcards "*/$edition_id.mmdb" > "$repo_dir/$edition_id.mmdb.tmp"

mv "$repo_dir/$edition_id.mmdb.tmp" "$repo_dir/$edition_id.mmdb"
