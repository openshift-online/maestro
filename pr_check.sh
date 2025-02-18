#!/bin/bash -ex

# Ensure git 2.9 is used
export PATH=/opt/rh/rh-git29/root/usr/bin/:$PATH
# Ensure the httpd 2.4 libraries are imported for libcurl to function properly in the git 2.9 SCL
export LD_LIBRARY_PATH=/opt/rh/httpd24/root/usr/lib64:$LD_LIBRARY_PATH

# Use same name for all instances.  This makes it easy to clean up a previously
# failed instance.  This assumes only one instance will be running at a time.
export IMAGE_NAME="test/maestro"

function cleanUp {
  if [ -n "${IMAGE_NAME:-}" ] && \
      [ -n "$(podman container ls -aqf name="${IMAGE_NAME}")" ] && \
      [ "$(podman container inspect -f '{{.State.Running}}' "${IMAGE_NAME}")" = "true" ]; then
    podman container kill "${IMAGE_NAME}"
  fi
}
trap cleanUp EXIT

test -f go1.23.6.linux-amd64.tar.gz || curl -O -J https://dl.google.com/go/go1.23.6.linux-amd64.tar.gz

podman build -t "$IMAGE_NAME" -f Dockerfile.test .

podman run -i "$IMAGE_NAME"
