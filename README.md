# What is this?

**[Drone](https://www.drone.io/) is an opensource CI server**. It allows you to run commands when you push to a Git repository, for example to build a release, run your unit tests, or publish a website.

Drone relies on **plugins called ["runners"](https://docs.drone.io/runner/overview/) to do the actual command execution**. It comes with runners for running on your machine (which is dangerous, commands in the repository could mess with your system) or on [cloud platforms](https://docs.drone.io/runner/vm/configuration/migration/) such as AWS and GCP.

For self-hosting purposes, being able to **run on virtual machines locally** is desirable for security, reproducibility, and cost. This is what this repository provides, using the [QEMU open-source emulator](https://www.qemu.org/).

# Installation

Download some images:

```console
$ qemu-images/download.sh
```

You can customize how images are run by editing the `.qemu.sh` scripts or making your own.

Download the qemu runner and configure to connect with your central Drone server using your server address and shared secret:

```console
$ docker run -d \
  --publish=3000:3000 \
  --env=DRONE_RPC_HOST=drone.example.com \
  --env=DRONE_RPC_PROTO=https \
  --env=DRONE_RPC_SECRET=${SECRET} \
  --env=DRONE_QEMU_IMAGE_DIR=/qemu-images \
  --env=DRONE_QEMU_DEFAULT_IMAGE=ubuntu-22.04 \
  --restart=always \
  --volume=$(pwd)/qemu-images:/qemu-images \
  --name=drone-runner-qemu ghcr.io/remram44/drone-runner-qemu
```

That's it. Go make some pipelines with `type: qemu`, they will be run by this system in their own, self-contained, ephemeral virtual machines.

# License

This software is licensed under the [Blue Oak Model License 1.0.0](https://spdx.org/licenses/BlueOak-1.0.0.html).

# Notice
<!-- do not remove notice -->

This software relies on the [runner-go](https://github.com/drone/runner-go) module authored by [Drone.IO, Inc](https://github.com/drone) under a [Non-compete](https://github.com/drone/runner-go/blob/master/LICENSE.md) license. This module can be used in any software for free for any permitted purpose in accordance with the license. This module cannot be used in any software that competes with the licensor.
