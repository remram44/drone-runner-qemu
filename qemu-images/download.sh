#!/bin/sh

set -eux

cd "$(dirname "$0")"

if ! [ -e "ubuntu-22.04.qcow2" ]; then
    curl -SLo "ubuntu-22.04.qcow2" https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img
fi

if ! [ -e "ubuntu-20.04.qcow2" ]; then
    curl -SLo "ubuntu-20.04.qcow2" https://cloud-images.ubuntu.com/focal/current/focal-server-cloudimg-amd64.img
fi

if ! [ -e "ubuntu-18.04.qcow2" ]; then
    curl -SLo "ubuntu-18.04.qcow2" https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img
fi

if ! [ -e "debian-12.qcow2" ]; then
    curl -SLo "debian-12.qcow2" https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2
fi

if ! [ -e "fedora-40.qcow2" ]; then
    curl -SLo "fedora-40.qcow2" https://download.fedoraproject.org/pub/fedora/linux/releases/40/Cloud/x86_64/images/Fedora-Cloud-Base-Generic.x86_64-40-1.14.qcow2
fi

if ! [ -e "alpine-3.19.qcow2" ]; then
    curl -SLo "alpine-3.19.qcow2" https://dl-cdn.alpinelinux.org/alpine/v3.19/releases/cloud/nocloud_alpine-3.19.1-x86_64-bios-cloudinit-r0.qcow2
fi
