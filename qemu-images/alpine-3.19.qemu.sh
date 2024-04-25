#!/bin/sh

if [ "x$QEMU_IMAGE" = x ] || [ "x$QEMU_SSH_PORT" = x ]; then
    exit 1
fi

exec qemu-system-x86_64 \
    -enable-kvm \
    -cpu host \
    -no-reboot \
    -drive "id=root,file=$QEMU_IMAGE,format=qcow2" \
    -drive "id=cidata,file=cloud-init.iso,media=cdrom" \
    -netdev "user,id=net0,hostfwd=tcp:127.0.0.1:$QEMU_SSH_PORT-:22" \
    -device virtio-net-pci,netdev=net0 \
    -device virtio-serial-pci \
    -nographic \
    -vga none \
    -display none \
    -m 1024 \
    -smp 2
