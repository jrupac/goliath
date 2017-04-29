# A script to build and package Goliath.

#!/usr/bin/env bash

set -euxo pipefail

echo "Building Goliath core."
if [ -f ./goliath ]
then
    rm ./goliath
fi
go build -x -v .

echo "Building Goliath frontend."
cd frontend/
if [ -d ./build/ ]
then
    rm -rf ./build/
fi
npm run build
cd ..

echo "Putting outputs in 'out/' folder."
if [ -d ./out/ ]
then
    rm -rf ./out/
fi
mkdir out
mkdir out/public
cp ./goliath ./out/
cp -R frontend/build/* out/public/
