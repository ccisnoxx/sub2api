# Deployment of This Fork

This directory belongs to [ccisnoxx/sub2api](https://github.com/ccisnoxx/sub2api). `main` holds deployment control tools; `personal` holds application source. Build application code from `personal` or an application release tag.

## Verified Deployment Methods

Last verified: **2026-10-10**. [v0.2.14-klno.5-tps.2](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.14-klno.5-tps.2) is a prerelease whose published artifact is the Linux amd64 image in `ghcr.io/ccisnoxx/sub2api`.

| Method | Applicable environment | Verification coverage |
|---|---|---|
| Docker Compose with local directories | Linux x86_64, Docker Engine and the Compose plugin; data in the deployment directory | Template parsing, image origin and configuration contracts; recovery evidence in [BASELINE.md](../BASELINE.md) |
| Docker Compose with named volumes | Same platform; data in Docker volumes | Template parsing, image origin and configuration contracts |
| Existing hostdzire deployment tool | Registered Linux x86_64 host with the fixed configuration | Records and offline contract tests in the [tool guide](personal/README.md); not an installer for new hosts |

Current releases have no binary attachments, Docker Hub publication target or native ARM64 image. Their installation tutorials have been removed. `install.sh` remains maintenance source and only downloads from this fork. The [Apple tool status](APPLE_CONTAINER.md) describes its development limitations. Source development instructions are in [README.md](../README.md#build-from-source-for-development).

## Docker Compose Installation

Clone the application release tag and use its matching fixed image digest. This historical tag still has an upstream image in its templates, so **include both `-f` arguments in every operation**. Current `main` control templates have corrected defaults; existing tags are not rewritten.

```bash
git clone --branch v0.2.14-klno.5-tps.2 --single-branch https://github.com/ccisnoxx/sub2api.git
cd sub2api/deploy
cp .env.example .env
chmod 600 .env
nano .env
cat > compose.personal.yml <<'YAML'
services:
  sub2api:
    image: ghcr.io/ccisnoxx/sub2api@sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530
    platform: linux/amd64
YAML
mkdir -p data postgres_data redis_data
docker compose -f docker-compose.local.yml -f compose.personal.yml config --images
docker compose -f docker-compose.local.yml -f compose.personal.yml pull
docker compose -f docker-compose.local.yml -f compose.personal.yml up -d
docker compose -f docker-compose.local.yml -f compose.personal.yml ps
```

Before startup, generate separate values for `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `JWT_SECRET` and `TOTP_ENCRYPTION_KEY` with `openssl rand -hex 32`. Configure your administrator privately and set `BIND_HOST=127.0.0.1`. Restrict `SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP` and `SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS` according to your needs, and configure an HTTPS reverse proxy for remote access. Keep `.env` and credentials in logs private.

For named volumes, replace `docker-compose.local.yml` with `docker-compose.yml` in every command and retain the image override. `docker-compose.standalone.yml` is a configuration reference for existing external PostgreSQL/Redis; a fresh installation has not been verified, so no standalone installation tutorial is provided. `docker-compose.dev.yml` is for source development, not published-image installation.

## Updates and Rollback

Back up the database and application data, and save the previous image digest, Compose files and `.env`. Select a target tag from this fork's [Releases](https://github.com/ccisnoxx/sub2api/releases), verify the source SHA, Linux amd64 image digest and migration compatibility, then update `image` in `compose.personal.yml`.

```bash
docker compose -f docker-compose.local.yml -f compose.personal.yml config --images
docker compose -f docker-compose.local.yml -f compose.personal.yml pull sub2api
docker compose -f docker-compose.local.yml -f compose.personal.yml up -d --no-deps sub2api
docker compose -f docker-compose.local.yml -f compose.personal.yml ps
```

Pulling the same digest does not upgrade the application. Use this image update path: current releases have no binary attachments for dashboard updates or binary installation. Database migrations are forward-only; rolling back an image does not reverse a migration. Recovery procedures and the verified recovery point are in [BASELINE.md](../BASELINE.md).

Supporting files include `.env.example`, `config.example.yaml`, the [image guide](DOCKER.md) and [edge security guide](EDGE_SECURITY.md). `docker-deploy.sh` remains a `main` template preparation tool that downloads this fork's control templates; it is not a release binary installer.

---

### How Auto-Setup Works

When using Docker Compose with `AUTO_SETUP=true`:

1. On first run, the system automatically:
   - Connects to PostgreSQL and Redis
   - Applies database migrations (SQL files in `backend/migrations/*.sql`) and records them in `schema_migrations`
   - Generates JWT secret (if not provided)
   - Creates admin account (email and password auto-generated if not provided; a provided password must be 8-72 bytes)
   - Writes config.yaml

2. No manual Setup Wizard needed - just configure `.env` and start

3. If `ADMIN_EMAIL` / `ADMIN_PASSWORD` are not set, check logs for the generated admin email (login username) and password:
   ```bash
   docker compose -f docker-compose.yml -f compose.personal.yml logs sub2api | grep "Generated admin"
   ```

### Startup and Database Recovery

Sub2API applies database migrations during application startup. PostgreSQL can
remain in its recovery/startup phase briefly after a host or Docker daemon
restart. The application retries transient PostgreSQL startup and connection
errors with bounded exponential backoff, then starts automatically when the
database becomes ready. Authentication errors, migration checksum mismatches,
SQL errors, and other permanent configuration or data errors fail immediately.

The Compose example also uses a PostgreSQL health check that verifies both
server readiness and a simple SQL query. `depends_on: condition: service_healthy`
controls dependency ordering for a fresh Compose start, but it is not a
replacement for application-level retries when Docker restores existing
containers after a host restart.

For systemd deployments, keep `Restart=always` and `RestartSec` configured in
`sub2api.service`; the application retry covers transient database startup,
while systemd remains the supervisor for permanent process exits. For
Kubernetes, use a PostgreSQL readiness probe and retain the Sub2API startup
retry behavior; configure the application liveness probe separately so a
database recovery period is not treated as a permanent process failure.

### Database Migration Notes (PostgreSQL)

- Migrations are applied in lexicographic order (e.g. `001_...sql`, `002_...sql`).
- `schema_migrations` tracks applied migrations (filename + checksum).
- Migrations are forward-only; rollback requires a DB backup restore or a manual compensating SQL script.

**Verify `users.allowed_groups` → `user_allowed_groups` backfill**

During the incremental GORM→Ent migration, `users.allowed_groups` (legacy `BIGINT[]`) is being replaced by a normalized join table `user_allowed_groups(user_id, group_id)`.

Run this query to compare the legacy data vs the join table:

```sql
WITH old_pairs AS (
  SELECT DISTINCT u.id AS user_id, x.group_id
  FROM users u
  CROSS JOIN LATERAL unnest(u.allowed_groups) AS x(group_id)
  WHERE u.allowed_groups IS NOT NULL
)
SELECT
  (SELECT COUNT(*) FROM old_pairs)           AS old_pair_count,
  (SELECT COUNT(*) FROM user_allowed_groups) AS new_pair_count;
```

### datamanagementd（数据管理）联动

如需启用管理后台“数据管理”功能，请额外部署宿主机 `datamanagementd`：

- 主进程固定探测 `/tmp/sub2api-datamanagement.sock`
- Docker 场景下需把宿主机 Socket 挂载到容器内同路径
- 详细步骤见：`deploy/DATAMANAGEMENTD_CN.md`

### Commands

For **local directory version** (docker-compose.local.yml):

```bash
# Start services
docker compose -f docker-compose.local.yml -f compose.personal.yml up -d

# Stop services
docker compose -f docker-compose.local.yml -f compose.personal.yml down

# View logs
docker compose -f docker-compose.local.yml -f compose.personal.yml logs -f sub2api

# Restart Sub2API only
docker compose -f docker-compose.local.yml -f compose.personal.yml restart sub2api

# Update after selecting the target fork digest in compose.personal.yml
docker compose -f docker-compose.local.yml -f compose.personal.yml pull
docker compose -f docker-compose.local.yml -f compose.personal.yml up -d

# Remove all data (caution!)
docker compose -f docker-compose.local.yml -f compose.personal.yml down
rm -rf data/ postgres_data/ redis_data/
```

For **named volumes version** (docker-compose.yml):

```bash
# Start services
docker compose -f docker-compose.yml -f compose.personal.yml up -d

# Stop services
docker compose -f docker-compose.yml -f compose.personal.yml down

# View logs
docker compose -f docker-compose.yml -f compose.personal.yml logs -f sub2api

# Restart Sub2API only
docker compose -f docker-compose.yml -f compose.personal.yml restart sub2api

# Update after selecting the target fork digest in compose.personal.yml
docker compose -f docker-compose.yml -f compose.personal.yml pull
docker compose -f docker-compose.yml -f compose.personal.yml up -d

# Remove all data (caution!)
docker compose -f docker-compose.yml -f compose.personal.yml down -v
```

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `POSTGRES_PASSWORD` | **Yes** | - | PostgreSQL password |
| `JWT_SECRET` | **Recommended** | *(auto-generated)* | JWT secret (fixed for persistent sessions) |
| `TOTP_ENCRYPTION_KEY` | **Recommended** | *(auto-generated)* | TOTP encryption key (fixed for persistent 2FA) |
| `SERVER_PORT` | No | `8080` | Server port |
| `ADMIN_EMAIL` | No | *(auto-generated)* | Admin email (login username) |
| `ADMIN_PASSWORD` | No | *(auto-generated)* | Admin password (8-72 bytes) |
| `TZ` | No | `Asia/Shanghai` | Timezone |
| `UPDATE_GITHUB_TOKEN` | No | *(empty)* | Token for `api.github.com` release checks only; asset downloads remain anonymous. |
| `GEMINI_OAUTH_CLIENT_ID` | No | *(builtin)* | Google OAuth client ID (Gemini OAuth). Leave empty to use the built-in Gemini CLI client. |
| `GEMINI_OAUTH_CLIENT_SECRET` | No | *(builtin)* | Google OAuth client secret (Gemini OAuth). Leave empty to use the built-in Gemini CLI client. |
| `GEMINI_OAUTH_SCOPES` | No | *(default)* | OAuth scopes (Gemini OAuth) |
| `GEMINI_QUOTA_POLICY` | No | *(empty)* | JSON overrides for Gemini local quota simulation (Code Assist only). |

See `.env.example` for all available options.

> **Note:** The `docker-deploy.sh` script automatically generates `JWT_SECRET`, `TOTP_ENCRYPTION_KEY`, and `POSTGRES_PASSWORD` for you.

### Easy Migration (Local Directory Version)

When using `docker-compose.local.yml`, all data is stored in local directories, making migration simple:

```bash
# On source server: Stop services and create archive
cd /path/to/deployment
docker compose -f docker-compose.local.yml -f compose.personal.yml down
cd ..
tar czf sub2api-complete.tar.gz deployment/

# Transfer to new server
scp sub2api-complete.tar.gz user@new-server:/path/to/destination/

# On new server: Extract and start
tar xzf sub2api-complete.tar.gz
cd deployment/
docker compose -f docker-compose.local.yml -f compose.personal.yml up -d
```

Your entire deployment (configuration + data) is migrated!

---

## Gemini OAuth Configuration

Sub2API supports three methods to connect to Gemini:

### Method 1: Code Assist OAuth (Recommended for GCP Users)

**No configuration needed** - always uses the built-in Gemini CLI OAuth client (public).

1. Leave `GEMINI_OAUTH_CLIENT_ID` and `GEMINI_OAUTH_CLIENT_SECRET` empty
2. In the Admin UI, create a Gemini OAuth account and select **"Code Assist"** type
3. Complete the OAuth flow in your browser

> Note: Even if you configure `GEMINI_OAUTH_CLIENT_ID` / `GEMINI_OAUTH_CLIENT_SECRET` for AI Studio OAuth,
> Code Assist OAuth will still use the built-in Gemini CLI client.

**Requirements:**
- Google account with access to Google Cloud Platform
- A GCP project (auto-detected or manually specified)

**How to get Project ID (if auto-detection fails):**
1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Click the project dropdown at the top of the page
3. Copy the Project ID (not the project name) from the list
4. Common formats: `my-project-123456` or `cloud-ai-companion-xxxxx`

### Method 2: AI Studio OAuth (For Regular Google Accounts)

Requires your own OAuth client credentials.

**Step 1: Create OAuth Client in Google Cloud Console**

1. Go to [Google Cloud Console - Credentials](https://console.cloud.google.com/apis/credentials)
2. Create a new project or select an existing one
3. **Enable the Generative Language API:**
   - Go to "APIs & Services" → "Library"
   - Search for "Generative Language API"
   - Click "Enable"
4. **Configure OAuth Consent Screen** (if not done):
   - Go to "APIs & Services" → "OAuth consent screen"
   - Choose "External" user type
   - Fill in app name, user support email, developer contact
   - Add scopes: `https://www.googleapis.com/auth/generative-language.retriever` (and optionally `https://www.googleapis.com/auth/cloud-platform`)
   - Add test users (your Google account email)
5. **Create OAuth 2.0 credentials:**
   - Go to "APIs & Services" → "Credentials"
   - Click "Create Credentials" → "OAuth client ID"
   - Application type: **Web application** (or **Desktop app**)
   - Name: e.g., "Sub2API Gemini"
   - Authorized redirect URIs: Add `http://localhost:1455/auth/callback`
6. Copy the **Client ID** and **Client Secret**
7. **⚠️ Publish to Production (IMPORTANT):**
   - Go to "APIs & Services" → "OAuth consent screen"
   - Click "PUBLISH APP" to move from Testing to Production
   - **Testing mode limitations:**
     - Only manually added test users can authenticate (max 100 users)
     - Refresh tokens expire after 7 days
     - Users must be re-added periodically
   - **Production mode:** Any Google user can authenticate, tokens don't expire
   - Note: For sensitive scopes, Google may require verification (demo video, privacy policy)

**Step 2: Configure Environment Variables**

```bash
GEMINI_OAUTH_CLIENT_ID=your-client-id.apps.googleusercontent.com
GEMINI_OAUTH_CLIENT_SECRET=GOCSPX-your-client-secret

# 可选：如需使用 Gemini CLI 内置 OAuth Client（Code Assist / Google One）
# 安全说明：本仓库不会内置该 client_secret，请在运行环境通过环境变量注入。
# GEMINI_CLI_OAUTH_CLIENT_SECRET=GOCSPX-your-built-in-secret
```

**Step 3: Create Account in Admin UI**

1. Create a Gemini OAuth account and select **"AI Studio"** type
2. Complete the OAuth flow
   - After consent, your browser will be redirected to `http://localhost:1455/auth/callback?code=...&state=...`
   - Copy the full callback URL (recommended) or just the `code` and paste it back into the Admin UI

### Method 3: API Key (Simplest)

1. Go to [Google AI Studio](https://aistudio.google.com/app/apikey)
2. Click "Create API key"
3. In Admin UI, create a Gemini **API Key** account
4. Paste your API key (starts with `AIza...`)

### Comparison Table

| Feature | Code Assist OAuth | AI Studio OAuth | API Key |
|---------|-------------------|-----------------|---------|
| Setup Complexity | Easy (no config) | Medium (OAuth client) | Easy |
| GCP Project Required | Yes | No | No |
| Custom OAuth Client | No (built-in) | Yes (required) | N/A |
| Rate Limits | GCP quota | Standard | Standard |
| Best For | GCP developers | Regular users needing OAuth | Quick testing |

---

## Troubleshooting

### Docker

For **local directory version**:

```bash
# Check container status
docker compose -f docker-compose.local.yml -f compose.personal.yml ps

# View detailed logs
docker compose -f docker-compose.local.yml -f compose.personal.yml logs --tail=100 sub2api

# Check database connection
docker compose -f docker-compose.local.yml -f compose.personal.yml exec postgres pg_isready

# Check Redis connection
docker compose -f docker-compose.local.yml -f compose.personal.yml exec redis redis-cli ping

# Restart all services
docker compose -f docker-compose.local.yml -f compose.personal.yml restart

# Check data directories
ls -la data/ postgres_data/ redis_data/
```

For **named volumes version**:

```bash
# Check container status
docker compose -f docker-compose.yml -f compose.personal.yml ps

# View detailed logs
docker compose -f docker-compose.yml -f compose.personal.yml logs --tail=100 sub2api

# Check database connection
docker compose -f docker-compose.yml -f compose.personal.yml exec postgres pg_isready

# Check Redis connection
docker compose -f docker-compose.yml -f compose.personal.yml exec redis redis-cli ping

# Restart all services
docker compose -f docker-compose.yml -f compose.personal.yml restart
```

### Common Issues

1. **Port already in use**: Change `SERVER_PORT` in `.env` or systemd config
2. **Database connection failed**: Check PostgreSQL is running and credentials are correct
3. **Redis connection failed**: Check Redis is running and password is correct
4. **Permission denied**: Check ownership of the container data directories

---

## TLS Fingerprint Configuration

Sub2API supports TLS fingerprint simulation to make requests appear as if they come from the official Claude CLI (Node.js client).

> **💡 Tip:** Visit **[tls.sub2api.org](https://tls.sub2api.org/)** to get TLS fingerprint information for different devices and browsers.

### Default Behavior

- Built-in `claude_cli_v2` profile simulates Node.js 20.x + OpenSSL 3.x
- JA3 Hash: `1a28e69016765d92e3b381168d68922c`
- JA4: `t13d5911h1_a33745022dd6_1f22a2ca17c4`
- Profile selection: `accountID % profileCount`

### Configuration

```yaml
gateway:
  tls_fingerprint:
    enabled: true  # Global switch
    profiles:
      # Simple profile (uses default cipher suites)
      profile_1:
        name: "Profile 1"

      # Profile with custom cipher suites (use compact array format)
      profile_2:
        name: "Profile 2"
        cipher_suites: [4866, 4867, 4865, 49199, 49195, 49200, 49196]
        curves: [29, 23, 24]
        point_formats: 0

      # Another custom profile
      profile_3:
        name: "Profile 3"
        cipher_suites: [4865, 4866, 4867, 49199, 49200]
        curves: [29, 23, 24, 25]
```

### Profile Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Display name (required) |
| `cipher_suites` | []uint16 | Cipher suites in decimal. Empty = default |
| `curves` | []uint16 | Elliptic curves in decimal. Empty = default |
| `point_formats` | []uint8 | EC point formats. Empty = default |

### Common Values Reference

**Cipher Suites (TLS 1.3):** `4865` (AES_128_GCM), `4866` (AES_256_GCM), `4867` (CHACHA20)

**Cipher Suites (TLS 1.2):** `49195`, `49196`, `49199`, `49200` (ECDHE variants)

**Curves:** `29` (X25519), `23` (P-256), `24` (P-384), `25` (P-521)
