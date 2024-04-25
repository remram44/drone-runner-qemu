#!/bin/sh

cd "$(dirname "$0")"
DEST="$(pwd)"
TEMPDIR="$(mktemp -d)"
cd "$TEMPDIR"

# Generate SSH key pair
ssh-keygen -q -N "" -f "$TEMPDIR/id_rsa"

# Build cloud-init volume ISO
printf 'instance-id: drone-qemu\nlocal-hostname: drone-qemu\n' > meta-data
printf '#cloud-config\npassword: %s\nchpasswd:\n  expire: false\nssh_pwauth: false\nssh_authorized_keys:\n  - %s\n' "$(uuidgen)" "$(cat "$TEMPDIR/id_rsa.pub")" > user-data
genisoimage -output "$DEST/cloud-init.iso" -volid cidata -joliet -rock user-data meta-data

cp id_rsa "$DEST/id_rsa"

# Clean up
cd /
rm -rf "$TEMPDIR"
