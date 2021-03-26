#!/bin/bash
set -e

repo_dir=`dirname "$0"`"/.."

static_dir="${1%/}"

function usage {
    echo "usage: $0 static_dir [--restart]"
}
if [ "$0" == "-h" ] || [ "$0" == "--help" ]; then usage; exit 2; fi
if [ "$static_dir" == "" ]; then usage; exit 1; fi

restart=false
for i in "$@"; do
    case $i in
        --restart) restart=true;;
    esac
done

find "$repo_dir/www/dist" -regex '.*\.\(js\|css\)$' -exec gzip -k5f "{}" \;
cp -r "$repo_dir/www/dist" "$static_dir"

if [ $restart = true ]; then
    for name in storjnet-http storjnet-update storjnet-tgbot storjnet-probe; do
        sudo systemctl restart $name
    done
fi
