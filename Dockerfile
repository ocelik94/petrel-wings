FROM golang:1.23-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/wings ./cmd/wings

FROM alpine:latest

RUN addgroup -S petrel && adduser -S -G petrel petrel
WORKDIR /app
COPY --from=builder /out/wings /usr/local/bin/wings

EXPOSE 8443
USER petrel
ENTRYPOINT ["/usr/local/bin/wings"]
