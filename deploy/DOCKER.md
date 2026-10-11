# Container Image of This Fork

Image repository: `ghcr.io/ccisnoxx/sub2api`. Last verified: **2026-10-10**. The current release path publishes Linux amd64 only; Docker Hub images and ARM64 variants are not published.

Verified release [v0.2.14-klno.5-tps.2](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.14-klno.5-tps.2) is a prerelease. Its source SHA is `3d5e1fde82707900a21f2b5538b112c7bab3c04a` and its fixed image is:

```bash
docker pull ghcr.io/ccisnoxx/sub2api@sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530
```

For a new installation, follow the [Docker Compose guide](README.md#docker-compose-installation), check out the application tag and override the historical template's image. Configuration uses separate `DATABASE_HOST`, `DATABASE_PASSWORD`, `REDIS_HOST` and related variables from [.env.example](.env.example). The unverified single-container example using `DATABASE_URL` / `REDIS_URL` has been removed.

Personal image tags omit `v`, for example `0.2.14-klno.5-tps.2`; `latest` is movable. Use a matching fixed digest for deployment and rollback. Do not assume upstream major/minor or architecture-suffix tags exist.

Before updating, verify the [Release](https://github.com/ccisnoxx/sub2api/releases), source SHA, OCI revision/source and platform, then follow [Updates and Rollback](README.md#updates-and-rollback). Verification covers anonymous GHCR metadata, Compose parsing and configuration contracts; this task does not run a fresh installation or application E2E.
