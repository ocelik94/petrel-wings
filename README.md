# Petrel Wings

Petrel Wings is the Go daemon agent for the Petrel game server management platform. It runs on host nodes and manages game server containers, console streams, file operations, and resource reporting for the panel.

## Prerequisites

- Go 1.23+
- Docker

## Quick Start

```bash
cp config.example.yml config.yml
make build
./bin/wings --config ./config.yml
```

You can also set `WINGS_CONFIG=/path/to/config.yml`.

## Configuration

```yaml
panel_url: "https://panel.example.com"
token: "shared-secret-daemon-token"

api:
  host: "0.0.0.0"
  port: "8443"
  tls_cert: "/etc/petrel/certs/cert.pem"
  tls_key: "/etc/petrel/certs/key.pem"

data_path: "/var/lib/petrel"

docker:
  socket: "/var/run/docker.sock"
  network: "petrel_network"
```

Defaults:
- `api.host`: `0.0.0.0`
- `api.port`: `8443`
- `data_path`: `/var/lib/petrel`
- `docker.socket`: `/var/run/docker.sock`

`token` is required.

## API Endpoints

- `GET /api/health`
- `GET /api/servers`
- `GET /api/servers/{id}`
- `POST /api/servers`
- `DELETE /api/servers/{id}`
- `POST /api/servers/{id}/power`
- `GET /api/servers/{id}/ws`
- `GET /api/servers/{id}/files/list`
- `GET /api/servers/{id}/files/contents`
- `POST /api/servers/{id}/files/write`
- `POST /api/servers/{id}/files/delete`

All endpoints except `/api/health` require `Authorization: Bearer <token>`.

## Development

```bash
make test
make build
make lint
```

## Docker Deployment

### Production deployment (with Traefik + automatic TLS)

Traefik terminates TLS and auto-provisions Let's Encrypt certificates, so you do not need to generate or mount manual `cert.pem`/`key.pem` files for Wings in the default production setup.

1. Clone both repositories side-by-side:

   ```text
   /opt/petrel/
   ├── petrel-panel/    # git clone of panel repo
   └── petrel-wings/    # git clone of wings repo (compose files live here)
   ```

2. In your `petrel-wings` directory, copy env template and edit values:

   ```bash
   cp .env.example .env
   ```

3. Edit `.env` with real domains, secrets, and database credentials.
4. Edit `wings-config.yml` (`panel_url`, `token`, and other values as needed).
5. Start the full stack:

   ```bash
   docker compose up -d
   ```

### Local development (no TLS)

Run:

```bash
docker compose -f docker-compose.dev.yml up -d
```

- Panel: http://localhost:3000
- Wings: http://localhost:8443

### TLS behavior

- Traefik sits in front of Panel and Wings and terminates TLS.
- Wings runs plain HTTP internally (`tls_cert` and `tls_key` are empty in provided configs).
- Let's Encrypt certs are stored automatically in the `letsencrypt_data` Docker volume.

For self-signed cert testing, generate certs:

```bash
openssl req -x509 -nodes -newkey rsa:2048 -days 365 \
  -keyout key.pem -out cert.pem -subj "/CN=wings.local"
```

Then mount them into Wings (for example `./certs:/etc/petrel/certs:ro`) and set `api.tls_cert` / `api.tls_key` in the Wings config to `/etc/petrel/certs/cert.pem` and `/etc/petrel/certs/key.pem`.

### Docker socket access

Wings requires `/var/run/docker.sock` from the host so it can create, start, stop, and inspect game server containers. This is the same Docker socket pattern used by tools like Portainer and Traefik.
