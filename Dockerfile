# syntax=docker/dockerfile:1
# Docker-Image für Coolify (wird von der GitHub-Action mit fertigen, ggf. signierten Dateien gebaut).
#   docker build -t nicolaj-it-software .   (lokal: vorher ./build.sh ausführen)
FROM alpine:3.20
ARG TARGETARCH
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H -u 10001 nrmm \
 && mkdir /data && chown nrmm /data
COPY --chmod=0755 dist/nicolaj-rmm-server-linux-${TARGETARCH} /usr/local/bin/nicolaj-rmm-server
ENV NRMM_ADDR=:8080 \
    NRMM_DATA=/data \
    NRMM_NO_TLS=1 \
    NRMM_BEHIND_PROXY=1 \
    NRMM_CONTAINER=1 \
    TZ=Europe/Berlin
USER nrmm
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/nicolaj-rmm-server"]
