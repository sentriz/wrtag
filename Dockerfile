FROM alpine:3.18 AS builder-taglib
WORKDIR /tmp
COPY alpine/taglib/APKBUILD .
RUN apk update && \
    apk add --no-cache abuild doas && \
    echo "permit nopass root" >/etc/doas.conf && \
    abuild-keygen -a -n -i && \
    REPODEST=/pkgs abuild -F -r

FROM golang:1.22-alpine AS builder
RUN apk add -U --no-cache \
    build-base \
    ca-certificates \
    git \
    zlib-dev \
    go

# TODO: delete this block when taglib v2 is on alpine packages
COPY --from=builder-taglib /pkgs/*/*.apk /pkgs/
RUN apk add --no-cache --allow-untrusted /pkgs/*

WORKDIR /src
COPY . .
RUN  \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=linux go build -o /usr/local/bin/ ./cmd/...

FROM alpine:3.19
LABEL org.opencontainers.image.source https://github.com/sentriz/wrtag
RUN apk add -U --no-cache \
    rsgain \
    ca-certificates

COPY --from=builder \
    /usr/lib/libgcc_s.so.1 \
    /usr/lib/libstdc++.so.6 \
    /usr/lib/libtag.so.2 \
    /usr/lib/
COPY --from=builder /usr/local/bin/* /usr/local/bin/

CMD ["wrtagweb"]
