# Apple Container Tool Status

Last checked: **2026-10-10**. This fork's current [Release](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.14-klno.5-tps.2) only publishes Linux amd64 and has no native ARM64 artifact. Apple container installation, update and restore tutorials have been removed. Use [Docker Compose on Linux x86_64](README.md#docker-compose-installation) for new deployments.

`apple-container.sh` remains a development tool for Apple silicon, macOS 26 and Apple container 1.1.0 or later. Operators must explicitly set `APPLE_CONTAINER_SUB2API_IMAGE` to an ARM64 image they build from this fork's `personal` application source. Without an image, the script stops instead of selecting an upstream image. Existing environment files retain their values; `up` and `pull` reject the inherited `weishaw/sub2api:latest` template default until the operator replaces it.

This task verifies the lifecycle with a simulated container CLI and the rejection of a missing image. Native ARM64 application deployment, backup/restore and dashboard binary updates are unverified. Restore a public installation tutorial only after matching fork artifacts and actual runtime validation are available.
