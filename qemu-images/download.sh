#!/bin/sh

set -eux

cd "$(dirname "$0")"

if ! [ -e ubuntu-22.04.img ]; then
    curl -sSLo "ubuntu-22.04.img.xz" https://cdimage.ubuntu.com/ubuntu-server/jammy/daily-preinstalled/current/jammy-preinstalled-server-amd64.img.xz
    unxz "ubuntu-22.04.img.xz"
fi
