
#!/bin/bash
set -e
base=`dirname "$0"`"/.."

echo building server...
cd $base
go build -v

cd "www"
echo installing client deps...
npm install --omit=dev

echo cleanup...
rm -rf "dist"

echo building client...
npm run build
