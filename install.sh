# A script to build and package Goliath.

#!/usr/bin/env bash

set -euxo pipefail

echo "Building Goliath core."

echo "Compiling protobufs."
protoc -I admin/ admin/admin.proto --go_out=plugins=grpc:admin

buildTimestamp=`date +%s`
buildHash=`git rev-parse HEAD`
ldFlags="-X main.buildTimestamp=$buildTimestamp -X main.buildHash=$buildHash"
echo "Compling Goliath core."
go build -x -v -mod=mod -ldflags "$ldFlags" .

echo "Compiling Goliath frontend."
cd frontend/
npm run build
cd ..

echo "Putting outputs in 'out/' folder."
if [[ -d ./out/ ]]; then
    rm -rf ./out/
fi
mkdir -p out/public

cp ./goliath ./out/
cp -R frontend/build/* ./out/public/

cd ./out/
ln -s ../frontend/node_modules/@postlight/mercury-parser/cli.js mercury-parser
ln -s ../config.ini .