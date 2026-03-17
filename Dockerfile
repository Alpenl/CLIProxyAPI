# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor ./vendor

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOMAXPROCS=1 go build -trimpath -buildvcs=false -mod=vendor -p=1 -tags timetzdata -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" -o ./CLIProxyAPI ./cmd/server/

RUN mkdir -p /image-root/data && touch /image-root/data/.keep && chown -R 1000:1000 /image-root/data

FROM alpine:3.21

RUN addgroup -g 1000 -S app && adduser -D -H -u 1000 -G app app

COPY --from=builder /app/CLIProxyAPI /app/CLIProxyAPI
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /image-root/data /data

COPY config.example.yaml /app/config.example.yaml
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
COPY run-as-user.sh /app/run-as-user.sh

RUN chmod 0755 /app/CLIProxyAPI /app/docker-entrypoint.sh /app/run-as-user.sh && mkdir -p /data

WORKDIR /app

EXPOSE 8317

ENV TZ=Asia/Shanghai \
    WRITABLE_PATH=/data

ENTRYPOINT ["/app/docker-entrypoint.sh"]
