FROM ubuntu:24.04@sha256:224a1869083a311ef3f13648a154ba79832fbef6364d31493642ca03082da254
RUN apt-get update -qq && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      cloud-init openssh-server sudo && \
    rm -rf /var/lib/apt/lists/*
ENV ART_CLOUD_INIT_TEST_CONTAINER=1
CMD ["sh", "-ec", "python3 /repo/deploy/tests/test_cloud_init.py && python3 /repo/deploy/tests/test_install_release.py"]
