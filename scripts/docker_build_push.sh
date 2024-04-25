#!/bin/sh

VERSION=$(git describe | sed 's/^v//')
IMAGE=ghcr.io/remram44/drone-runner-qemu:$VERSION

docker buildx build --pull \
    . \
    --cache-from type=registry,ref=ghcr.io/remram44/drone-runner-qemu/buildxcache \
    --cache-to type=registry,mode=max,ref=ghcr.io/remram44/drone-runner-qemu/buildxcache,oci-mediatypes=false \
    --platform linux/amd64,linux/arm64,linux/arm/v7 \
    --push --tag $IMAGE
