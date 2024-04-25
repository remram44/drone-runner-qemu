#!/bin/sh

set -eux

cd "$(dirname "$0")"

if ! [ -e "ubuntu-22.04.qcow2" ]; then
    curl -SLo "ubuntu-22.04.qcow2" https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img
fi
