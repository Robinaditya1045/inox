# Oracle Cloud deployment

Single-VM deployment of the Inox backend, worker and admin portal onto an Oracle
Cloud Ampere instance (arm64, 1 OCPU / 6 GB, Oracle Linux 9). The viewer client
stays on Vercel and talks to this host over HTTPS.

```
  Vercel (frontend)                       browser
        |                                    |
        | https://<host>/api/v1              | https://<host>/admin/
        | wss://<host>/api/v1/rooms/{id}/ws  |
        v                                    v
  +---------------------- Oracle VM -----------------------+
  |  Caddy :80 :443  (host net, auto TLS)                  |
  |     /admin/*  -> /srv/admin  (static admin bundle)     |
  |     everything else -> 127.0.0.1:8080                  |
  |                                                        |
  |  backend :8080 (host net)   worker (host net)          |
  |  SFU media: UDP 50000-50100 (host net, NAT1to1)        |
  |                                                        |
  |  postgres / redis / minio -- bridge net,               |
  |  published on 127.0.0.1 only                           |
  +--------------------------------------------------------+
```

## Why the networking is split

`backend`, `worker` and `caddy` use `network_mode: host`; the datastores use a
bridge network published on loopback.

The SFU is the reason. Docker's userland proxy relays published UDP ports, which
rewrites the source address of every RTP packet — ICE connectivity checks then
fail, and voice chat hangs in `checking` forever. Host networking gives Pion the
real socket. Keeping Postgres, Redis and MinIO on `127.0.0.1` means they are
reachable from the host-networked containers but never from the internet, no
matter what Docker writes into iptables.

## First-time setup

1. **Host prep** (installs Docker, opens the host firewall):

   ```bash
   scp deploy/oracle/bootstrap.sh opc@<ip>:~ && ssh opc@<ip> 'bash ~/bootstrap.sh'
   ```

2. **OCI VCN security list** — the host firewall is only half of it. In the
   console, under *Networking > Virtual Cloud Networks > \<VCN\> > Security
   Lists*, allow ingress from `0.0.0.0/0`:

   | Protocol | Port          | Purpose                          |
   | -------- | ------------- | -------------------------------- |
   | TCP      | 80            | HTTP + Let's Encrypt validation  |
   | TCP      | 443           | HTTPS and WSS                    |
   | UDP      | 50000 – 50100 | WebRTC media (voice/screenshare) |

3. **Secrets** — on the server:

   ```bash
   cd ~/inox/deploy/oracle
   cp .env.prod.example .env.prod && chmod 600 .env.prod
   # fill in: PUBLIC_HOST, PUBLIC_IP, passwords, SESSION_SECRET (>= 32 chars),
   # CORS_ALLOWED_ORIGINS, ADMIN_EMAILS, WEBRTC_PUBLIC_IP
   ```

   `.env.prod` is git-ignored and never overwritten by `deploy.sh`; it is the one
   file that lives only on the server.

4. **Deploy.** Push to `main` and CI does the rest (see below). For the very
   first deploy, before the secret is configured, run it by hand:

   ```bash
   ./deploy/oracle/deploy.sh
   ```

## Continuous deployment

Pushing to `main` deploys. `.github/workflows/ci.yml` runs lint/test/typecheck,
builds the backend image on a native `ubuntu-24.04-arm` runner, publishes it to
`ghcr.io/<owner>/inox-backend` tagged with the commit SHA, then over SSH pulls
that tag on the host and restarts the stack. Pull requests build and test but
never deploy.

The image is deliberately not built on the target. The instance has one Ampere
core, where the same build takes minutes and a failure would leave production
half-built. The host only pulls, and `--no-build` makes a missing image fail
loudly rather than silently falling back to compiling there.

Setup is one repository secret:

```bash
gh secret set ORACLE_SSH_KEY < ~/.ssh/inox_gha_deploy
```

Everything else (host, user, public hostname) is non-secret and lives in the
workflow's `env:` block. The host key is pinned there too, so a redirected
deploy cannot hand the key to a different machine. The host authenticates to
the registry with the job's short-lived `GITHUB_TOKEN`, piped over SSH stdin so
it never reaches the process list, and logs out again at the end — no personal
access token is stored anywhere.

### Rolling back

Each deploy is pinned to a commit, and the host records what it is running in
`BACKEND_IMAGE` inside `.env.prod`. To go back:

```bash
ssh opc@<host>
cd ~/inox/deploy/oracle
sed -i 's|^BACKEND_IMAGE=.*|BACKEND_IMAGE=ghcr.io/<owner>/inox-backend:<older-sha>|' .env.prod
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --no-build
```

Re-running the workflow for an older commit does the same thing.

## Day-to-day

`deploy.sh` remains as the manual fallback for when CI is unavailable or you
need to ship something uncommitted. It builds on the host, so it is slower.

```bash
./deploy/oracle/deploy.sh              # build admin bundle, sync, rebuild, restart
SKIP_ADMIN=1 ./deploy/oracle/deploy.sh # backend only

# on the server, from ~/inox/deploy/oracle:
alias dc='docker compose --env-file .env.prod -f docker-compose.prod.yml'
dc ps
dc logs -f backend
dc restart backend
dc run --rm migrate --command=status --dir=/app/migrations
```

Migrations run as a one-shot `migrate` service that must exit 0 before the
backend starts — the server itself does not migrate on boot.

## Client configuration

The admin portal is built by `deploy.sh` and served from `/admin/` on this host,
same-origin with the API. The Vercel frontend needs these environment variables,
followed by a redeploy so Vite bakes them in:

```
VITE_API_BASE_URL=https://<host>/api/v1
VITE_WS_BASE_URL=wss://<host>/api/v1
VITE_MEDIA_STREAM_BASE_URL=https://<host>/media/stream
```

`CORS_ALLOWED_ORIGINS` is an exact-match list — no wildcards, no trailing
slashes. Vercel preview deployments get their own hostnames, so add them
explicitly if you need previews to reach the API.

## Gotchas

- **Admin access** is granted only by listing an address in `ADMIN_EMAILS`;
  there is no role column on `users`. Sign up with that exact address.
- **`PUBLIC_IP` / `WEBRTC_PUBLIC_IP`** must be the VM's public address. The
  instance only ever sees its private `10.0.0.x` address, so without this the
  SFU advertises candidates no remote browser can route to.
- **Certificates** live in the `caddy_data` volume. Don't delete it casually —
  Let's Encrypt rate-limits reissuance.
- **After a host reboot** the backend and worker restart a couple of times
  before settling. Docker starts containers by restart policy, which ignores the
  `depends_on` ordering that `compose up` honours, so they briefly race Postgres.
  `restart: unless-stopped` converges it — the stack was healthy ~20s after boot.
- **Disk** is 30 GB and shared with MinIO media. Watch `docker system df`.
