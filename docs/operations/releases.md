# Releases and activation

The release pipeline packages committed source and immutable Docker image identities. It is a single-instance replacement: web state is transient and the activation script never runs two independent production queues together.

## Build a reviewable release

From a clean committed checkout with Docker available:

```sh
make check
deploy/scripts/build-release.sh
```

The builder rejects dirty worktrees and existing output directories. It builds `linux/amd64` app and edge targets for the full Git revision. Output is `out/releases/<commit>/` containing `images.tar`, `images.env` with immutable `sha256:` image IDs, archived deployment files, `catalogue.json`, a tool/edition/source manifest and `SHA256SUMS`. Retain this directory unchanged. This command builds a release; it does not install it on a host.

The [verification workflow](../development/testing.md) covers additional runtime/browser/security checks. Catalogue artifacts are reviewed and promoted separately; `tools/webcatalog` does not change embedded assets automatically.

## Install on a configured host

[Provisioning](provisioning.md) invokes `provision-app.py`, which verifies that every file in the selected release is checksummed, uploads an archive with SSH, verifies the archive digest on the host and runs `/usr/local/sbin/art-install-release`. The helper is installed by cloud-init. It verifies the source/platform, installs under `/opt/art/releases/<commit>`, writes the selected origin to `/etc/art/operator.env`, records the installation selection in `/etc/art/deployment-approved`, and invokes activation.

For an already installed release, the host's root entry point is:

```sh
/opt/art/releases/<commit>/deploy/scripts/activate-release.sh <full-40-character-commit>
```

The angle-bracket values are placeholders. Select the intended retained commit and configured origin before executing. The script requires the approval marker already present on the host; this is an implementation guard, not an automatic remote deployment step performed by documentation builds.

## Activation sequence

1. Acquire the deployment lock and verify release checksums and immutable image IDs.
2. Load the archived images and validate Compose configuration.
3. Run smoke checks against a disposable isolated project.
4. Disable prior-generation admission and wait up to 40 seconds for its queue/running counts to reach zero.
5. Stop the prior web and renderer before replacing the `/opt/art/current` symlink.
6. Start the new Compose release without builds/pulls, wait for health, then check private readiness, web liveness and the canonical origin.

A failure before replacement re-enables prior generation. A failure after replacement stops the changed services and attempts to restore the prior symlink/release, or tears down the failed first installation. The script reports failure even after attempting recovery: inspect state and readiness. Installer failure also restores the prior operator environment where one existed. Release switching loses in-memory studio navigation regardless of success.

## Roll back deliberately

Select a retained previously verified release and run its activation entry point with that full commit. It follows the same checks, drain and readiness process. Keep operator origin and network configuration consistent with the target release. Cache keys include the build, so a rolled-back renderer will not confuse another build's pixels with its own.

Source: [build](../../deploy/scripts/build-release.sh), [install](../../deploy/scripts/install-release.sh), [activate](../../deploy/scripts/activate-release.sh), [smoke](../../deploy/scripts/smoke-release.sh), [release Compose wrapper](../../deploy/scripts/compose-release.sh).
