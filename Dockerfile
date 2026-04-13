FROM golang:1.23-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/wings ./cmd/wings

FROM alpine:latest

# ca-certificates needed for HTTPS calls to the panel
RUN apk add --no-cache ca-certificates

RUN addgroup -S petrel && adduser -S -G petrel petrel

# Create data and config directories
RUN mkdir -p /var/lib/petrel /etc/petrel && \
    chown -R petrel:petrel /var/lib/petrel /etc/petrel

WORKDIR /app
COPY --from=builder /out/wings /usr/local/bin/wings

EXPOSE 8443

# Wings needs access to /var/run/docker.sock (mounted from host).
# Do NOT switch to non-root user here; the socket requires root
# or docker-group membership. In production, add petrel to docker group.

ENTRYPOINT ["/usr/local/bin/wings"]
CMD ["--config", "/etc/petrel/config.yml"]
