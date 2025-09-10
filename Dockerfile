# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.25-alpine3.22 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN  \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/ ./cmd/...

FROM alpine:3.22 AS essentia-extractors
ARG TARGETARCH
RUN if [ "$TARGETARCH" = "amd64" ]; then \
      apk add --no-cache curl tar \
      && curl -L -o essentia-extractors.tar.gz https://essentia.upf.edu/extractors/essentia-extractors-v2.1_beta2-linux-x86_64.tar.gz \
      && tar -xzvf essentia-extractors.tar.gz; \
    fi

FROM alpine:3.22
LABEL org.opencontainers.image.source=https://github.com/sentriz/wrtag
RUN apk add -U --no-cache \
    su-exec \
    rsgain
COPY --from=builder /out/* /usr/local/bin/
COPY --from=essentia-extractors /tmp/streaming_extractor_music /usr/local/bin/
COPY docker-entry /
ENTRYPOINT ["/docker-entry"]
CMD ["wrtagweb"]
