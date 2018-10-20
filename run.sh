#!/usr/bin/env bash

set -euxo pipefail

if [ ! -f ./out/goliath ] || [ ! -d ./out/public/ ]; then
    echo "Binary does not exist. Have you built with install.sh?"
    exit 1
fi

# Goliath logs to /tmp/goliath.{INFO|WARNING|ERROR}.
# Redirect all logs related to nohup to /tmp/goliath.STDERR.
nohup ./out/goliath --config=./config.ini >/tmp/goliath.STDERR 2>&1 </dev/null &