Example images

You can use `download.sh` to download images, however note that:

- The fedora image doesn't include `git`, so the Drone `clone` step will fail unless you install it into the image
- The alpine image doesn't include `git` and additionally has a very small virtual size, you might want to resize it
