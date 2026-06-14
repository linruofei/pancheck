#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")"

nohup ./pancheck >/dev/null 2>&1 &
