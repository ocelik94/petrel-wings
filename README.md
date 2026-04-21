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

## Full-Stack Deployment

Full-stack Docker Compose orchestration (Panel + Wings + database + Traefik) is managed in the **[ocelik94/petrel-panel](https://github.com/ocelik94/petrel-panel)** repository. See that repo for compose files, `.env.example`, and startup instructions.

## Building the Wings Docker Image

To build the Wings daemon image locally:

```bash
make docker
# or
docker build -t petrel-wings:latest .
```

To run the Wings daemon container standalone (mount your own config and Docker socket):

```bash
docker run -d \
  --name wings \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /etc/petrel/config.yml:/etc/petrel/config.yml:ro \
  -p 8443:8443 \
  petrel-wings:latest
```

See `config.example.yml` for a complete Wings configuration reference.

### Docker socket access

Wings requires `/var/run/docker.sock` from the host so it can create, start, stop, and inspect game server containers. This is the same Docker socket pattern used by tools like Portainer and Traefik.
