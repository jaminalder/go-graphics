# Releases and activation

The release pipeline packages committed source and immutable images. It replaces one active build after draining River; unexpired studio state and pointers survive in the separate database. Builds include pinned PostgreSQL/local-S3 images for isolated smoke checks.

See [persistence operations](persistence.md) for initial data/credential bootstrap. Activation pauses admission, drains jobs, stops consumers, migrates, increments build epoch, starts matching services and restores prior admission. Rollback checks schema compatibility; it never down-migrates or automatically starts a legacy binary after persistence cutover. Installer failure does not override that recovery by blindly restarting a symlink.

The monitoring change adds application migration 2. It preserves version-1 data, but version-1 binaries' exact schema check rejects it. After that migration, rollback requires a compatible version-2 build; this release is not automatically rollback-compatible with `8ce7cd6`. The [monitoring runbook](monitoring.md) explains local rebuild and hosted command use.

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
4. Record prior admission policy, disable new admission and wait up to 360 one-second checks for outstanding jobs to reach zero (command time adds overhead).
5. Stop prior web/renderer, migrate application/River schemas and refresh runtime grants, then activate the new build/epoch and replace the symlink.
6. Start the release without builds/pulls, check health/readiness/origin, then restore prior admission policy.

A pre-switch failure restores prior admission where enabled. After switching, rollback requires a retained persistence-aware release passing schema checks and no unfinished work blocking epoch change. Legacy/incompatible fallback is refused; data is preserved. First-install failure tears down app services, not database volumes. The installer restores the previous operator environment on activation failure. Unexpired studio navigation survives persistent releases; only the initial legacy cutover resets it.

## Roll back deliberately

Select a retained previously verified release and run its activation entry point with that full commit. It follows the same checks, drain and readiness process. Keep operator origin and network configuration consistent with the target release. Cache keys include the build, so a rolled-back renderer will not confuse another build's pixels with its own.

Source: [build](../../deploy/scripts/build-release.sh), [install](../../deploy/scripts/install-release.sh), [activate](../../deploy/scripts/activate-release.sh), [smoke](../../deploy/scripts/smoke-release.sh), [release Compose wrapper](../../deploy/scripts/compose-release.sh).
