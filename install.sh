# A script to build and package Goliath.

#!/usr/bin/env bash

set -euxo pipefail

echo "Building Goliath core."
buildTimestamp=`date +%s`
buildHash=`git rev-parse HEAD`
ldFlags="-X main.buildTimestamp=$buildTimestamp -X main.buildHash=$buildHash"
go build -x -v -ldflags "$ldFlags" .

echo "Building Goliath frontend."
cd frontend/
npm run build
cd ..

echo "Putting outputs in 'out/' folder."
if [ -d ./out/ ]
then
    rm -rf ./out/
fi
mkdir -p out/public
cp ./goliath ./out/
cp -R frontend/build/* out/public/
